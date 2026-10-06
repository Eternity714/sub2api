import { readFile, writeFile } from 'node:fs/promises';

const base = process.env.SUB2API_TEST_BASE_URL;
const previewUrl = new URL(base);
if (!['127.0.0.1', 'localhost', 'host.docker.internal'].includes(previewUrl.hostname) || previewUrl.port !== '18081') {
  throw new Error('Setup only supports the isolated local preview on port 18081');
}
const adminHeaders = { 'x-api-key': process.env.SUB2API_ADMIN_KEY };
const secrets = [process.env.SUB2API_ADMIN_KEY, process.env.UPSTREAM_API_KEY, process.env.SUB2API_TEST_PASSWORD];
function safe(value) {
  let text = String(value);
  for (const secret of secrets) if (secret) text = text.split(secret).join('[REDACTED]');
  return text.replace(/(?:sk-|admin-)[a-zA-Z0-9_-]+/g, '[REDACTED]');
}
async function api(path, { method = 'GET', body, headers = adminHeaders } = {}) {
  const response = await fetch(base + '/api/v1' + path, {
    method, headers: { ...headers, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(30000),
  });
  const raw = await response.text();
  let payload;
  try { payload = JSON.parse(raw); } catch { throw new Error(`${method} ${path}: HTTP ${response.status}, invalid JSON`); }
  if (!response.ok || payload.code && payload.code !== 0) throw new Error(safe(`${method} ${path}: HTTP ${response.status} ${payload.message || payload.code}`));
  return payload.data ?? payload;
}

const model = process.env.SUB2API_TEST_MODEL;
const users = await api('/admin/users?page_size=100');
console.log('Admin API authentication verified; user count:', users.total);
if (process.env.SETUP_DRY_RUN === 'true') process.exit(0);
const names = { group: '媒体原生测试分组', channel: '媒体原生测试渠道', account: '媒体原生测试上游', key: '媒体原生端到端测试' };
const pricing = [{ platform: 'grsai', models: [model], billing_mode: 'image', per_request_price: 0.01, intervals: [] }];
let groups = await api('/admin/groups?page_size=100');
let group = groups.items.find(item => item.name === names.group);
if (group && group.platform !== 'grsai') throw new Error('Existing native media test group has an incompatible platform');
if (!group) group = await api('/admin/groups', { method: 'POST', body: {
  name: names.group, description: '本地 18081 nano-banana-2-lite 端到端测试，0.01 USD/张仅用于验证计费',
  platform: 'grsai', subscription_type: 'standard', rate_multiplier: 1, is_exclusive: false,
  allow_image_generation: true, model_pricing: pricing,
} });
console.log('GRS.AI balance media group ready:', group.id);
let channels = await api('/admin/channels?page_size=100');
let channel = channels.items.find(item => item.name === names.channel);
if (channel && (!channel.group_ids?.includes(group.id) || !channel.model_pricing?.some(item =>
  item.platform === 'grsai' && item.models?.includes(model)))) throw new Error('Existing native media channel has incompatible configuration');
if (!channel) channel = await api('/admin/channels', { method: 'POST', body: {
  name: names.channel, description: '本地 18081 测试渠道', group_ids: [group.id],
  billing_model_source: 'requested', restrict_models: true, model_pricing: pricing,
} });
console.log('GRS.AI media channel ready:', channel.id);
let accounts = await api('/admin/accounts?page_size=100');
let account = accounts.items.find(item => item.name === names.account);
if (account && (account.platform !== 'grsai' || account.type !== 'apikey' ||
  !account.group_ids?.includes(group.id))) throw new Error('Existing native media account has incompatible configuration');
if (!account) account = await api('/admin/accounts', { method: 'POST', body: {
  name: names.account, notes: '本地 18081 GRS.AI 上游，测试模型 nano-banana-2-lite',
  platform: 'grsai', type: 'apikey',
  credentials: { base_url: process.env.UPSTREAM_BASE_URL, api_key: process.env.UPSTREAM_API_KEY },
  group_ids: [group.id], concurrency: 2, priority: 1, rate_multiplier: 1,
  upstream_billing_probe_enabled: false,
} });
console.log('GRS.AI API Key upstream ready:', account.id);
let user = users.items.find(item => item.email === process.env.SUB2API_TEST_EMAIL);
if (!user) user = await api('/admin/users', { method: 'POST', body: {
  email: process.env.SUB2API_TEST_EMAIL, password: process.env.SUB2API_TEST_PASSWORD,
  username: '媒体测试用户', notes: '本地人工测试专用', balance: 10, concurrency: 2,
  allowed_groups: [group.id], role: 'user',
} });
if (!user.allowed_groups?.includes(group.id)) user = await api('/admin/users/' + user.id, { method: 'PUT', body: {
  allowed_groups: [...new Set([...(user.allowed_groups ?? []), group.id])],
} });
console.log('Test user ready:', user.id, 'balance:', user.balance);
await api('/admin/settings', { method: 'PUT', body: {
  available_channels_enabled: true, model_plaza_enabled: true, model_plaza_require_auth: false,
} });
const login = await api('/auth/login', { method: 'POST', headers: {}, body: {
  email: process.env.SUB2API_TEST_EMAIL, password: process.env.SUB2API_TEST_PASSWORD,
} });
const userHeaders = { Authorization: 'Bearer ' + login.access_token };
secrets.push(login.access_token, login.refresh_token);
const keys = await api('/keys?page_size=100', { headers: userHeaders });
let key = keys.items.find(item => item.name === names.key && item.group_id === group.id && item.status === 'active' &&
  (!item.expires_at || new Date(item.expires_at).getTime() > Date.now()));
if (!key) key = await api('/keys', { method: 'POST', headers: userHeaders, body: {
  name: names.key, group_id: group.id, quota: 1, expires_in_days: 7,
} });
if (!key.key) throw new Error('Created key missing usable key value');
secrets.push(key.key);
let initialBalance = user.balance;
try {
  const previous = JSON.parse(await readFile('.dev/media-preview/test-fixture.json', 'utf8'));
  if (previous.api_key_id === key.id && previous.group_id === group.id) initialBalance = previous.initial_balance;
} catch (error) {
  if (error.code !== 'ENOENT') throw new Error('Existing test fixture is unreadable; baseline was not overwritten');
}
// Retire only the exact OpenAI fixtures created by the previous local test.
// Keep their rows and completed task/billing history for comparison.
const previousGroup = groups.items.find(item => item.name === '媒体人工测试' && item.platform === 'openai');
const previousAccount = accounts.items.find(item => item.name === '媒体测试上游' && item.platform === 'openai');
const previousChannel = channels.items.find(item => item.name === '媒体测试渠道' &&
  item.group_ids?.includes(previousGroup?.id) && item.model_pricing?.some(price => price.platform === 'openai'));
for (const [resource, previous] of [['accounts', previousAccount], ['channels', previousChannel], ['groups', previousGroup]]) {
  if (previous?.status === 'active') {
    const status = resource === 'channels' ? 'disabled' : 'inactive';
    await api(`/admin/${resource}/${previous.id}`, { method: 'PUT', body: { status } });
    console.log('Historical OpenAI test fixture retired:', resource, previous.id);
  }
}
await writeFile('.dev/media-preview/media-test.env', [
  'SUB2API_TEST_BASE_URL=http://host.docker.internal:18081',
  'SUB2API_TEST_MODEL=' + model, 'SUB2API_TEST_KEY=' + key.key,
  'SUB2API_ADMIN_KEY=' + process.env.SUB2API_ADMIN_KEY,
  'SUB2API_TEST_USER_ID=' + user.id, 'SUB2API_TEST_API_KEY_ID=' + key.id,
  'SUB2API_TEST_EXPECTED_COST=0.01',
  'SUB2API_TEST_INITIAL_BALANCE=' + initialBalance,
].join('\n') + '\n', { mode: 0o600 });
await writeFile('.dev/media-preview/test-fixture.json', JSON.stringify({
  group_id: group.id, channel_id: channel.id, account_id: account.id, user_id: user.id,
  api_key_id: key.id, model, initial_balance: initialBalance, unit_price_usd: 0.01,
}, null, 2) + '\n');
await writeFile('.dev/media-preview/test-login.txt', [
  '测试用户网址：http://127.0.0.1:18081/login', '测试用户邮箱：' + process.env.SUB2API_TEST_EMAIL,
  '测试用户密码：' + process.env.SUB2API_TEST_PASSWORD, '调用 Key 见同目录 media-test.env',
].join('\n') + '\n', { mode: 0o600 });
console.log('Test Sub2API Key ready:', key.id, '(secret stored in ignored local env)');
