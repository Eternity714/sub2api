import { randomUUID } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';

// Run with the local Node 20 Compose tools service. This script has no media
// generation endpoint and never requests the configured upstream URL.
const reportFile = new URL('./results/config-report.json', import.meta.url);
const report = { passed: false, checks: [] };
const secrets = new Set();
const detailFields = new Set([
  'account_id', 'group_id', 'channel_id', 'user_id', 'api_key_id', 'created_id',
  'platform', 'model', 'price_usd', 'http_status', 'expected_http_status',
  'present', 'matches', 'valid', 'custom', 'redacted', 'bound', 'enabled',
  'standard', 'exclusive', 'requested', 'restricted', 'active', 'no_task_data',
  'provider_hidden',
]);
let baseURL;
let adminKey;
let testKey;
let userToken;
let allowedRequests;

class CheckFailure extends Error {
  constructor(reason) {
    super(reason);
    this.reason = reason;
  }
}

function requireValue(condition, reason) {
  if (!condition) throw new CheckFailure(reason);
}

function scrub(value) {
  if (typeof value === 'string') {
    for (const secret of secrets) {
      if (secret) value = value.split(secret).join('[REDACTED]');
    }
    return value;
  }
  if (Array.isArray(value)) return value.map(scrub);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, scrub(item)]));
  }
  return value;
}

async function persist() {
  await mkdir(new URL('./results/', import.meta.url), { recursive: true });
  await writeFile(reportFile, `${JSON.stringify(scrub(report), null, 2)}\n`, { mode: 0o600 });
}

async function check(name, condition, details = {}) {
  requireValue(/^[a-z0-9_]+$/.test(name), 'INVALID_CHECK_NAME');
  for (const [key, value] of Object.entries(details)) {
    requireValue(detailFields.has(key), 'UNSAFE_REPORT_FIELD');
    requireValue(
      typeof value === 'boolean' || (typeof value === 'number' && Number.isFinite(value)) ||
      (key === 'platform' && ['openai', 'grsai', 'media'].includes(value)) ||
      (key === 'model' && value === report.model),
      'UNSAFE_REPORT_VALUE',
    );
  }
  const item = { check: name, passed: Boolean(condition), ...details };
  report.checks.push(item);
  await persist();
  console.log(JSON.stringify(scrub(item)));
}

async function loadEnvironment() {
  let local = '';
  try {
    local = await readFile(new URL('../.dev/media-preview/media-test.env', import.meta.url), 'utf8');
  } catch (error) {
    requireValue(error.code === 'ENOENT', 'ENV_FILE_READ_FAILED');
  }
  const fallback = {};
  for (const line of local.split(/\r?\n/)) {
    const match = /^([A-Z][A-Z0-9_]*)=(.*)$/.exec(line);
    if (match) fallback[match[1]] = match[2];
  }
  return { ...fallback, ...process.env };
}

function positiveID(value) {
  return Number.isSafeInteger(value) && value > 0;
}

function isObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function configuredURL(value) {
  try {
    const url = new URL(value);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) return null;
    return url;
  } catch {
    return null;
  }
}

function normalizedURL(value) {
  return configuredURL(value)?.href.replace(/\/+$/, '');
}

async function readJSON(response) {
  requireValue(response.body, 'EMPTY_RESPONSE');
  const reader = response.body.getReader();
  const chunks = [];
  let length = 0;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      length += value.byteLength;
      requireValue(length <= 4 * 1024 * 1024, 'RESPONSE_TOO_LARGE');
      chunks.push(Buffer.from(value));
    }
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
  try {
    return JSON.parse(Buffer.concat(chunks, length).toString('utf8'));
  } catch {
    throw new CheckFailure('INVALID_JSON');
  }
}

async function request(path, { method = 'GET', body, auth = 'admin' } = {}) {
  requireValue(allowedRequests.has(`${method} ${path}`), 'REQUEST_NOT_ALLOWED');
  const url = new URL(path, baseURL);
  requireValue(url.origin === baseURL.origin, 'TARGET_ORIGIN_CHANGED');
  let response;
  try {
    response = await fetch(url, {
      method,
      redirect: 'error',
      headers: {
        Accept: 'application/json',
        ...(auth === 'admin' ? { 'x-api-key': adminKey } : {}),
        ...(auth === 'key' ? { Authorization: `Bearer ${testKey}` } : {}),
        ...(auth === 'user' ? { Authorization: `Bearer ${userToken}` } : {}),
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(30000),
    });
    return { status: response.status, payload: await readJSON(response) };
  } catch (error) {
    if (error instanceof CheckFailure) throw error;
    throw new CheckFailure('LOCAL_REQUEST_FAILED');
  }
}

async function readAdmin(name, path, id, idField) {
  const { status, payload } = await request(path);
  const data = payload?.data;
  const valid = status === 200 && payload?.code === 0 && isObject(data) && data.id === id;
  await check(name, valid, { http_status: status, [idField]: id });
  requireValue(valid, 'ADMIN_CONFIG_READ_FAILED');
  return data;
}

async function checkPricing(name, pricing, model, price, ids) {
  const matching = Array.isArray(pricing) ? pricing.filter(item => item?.models?.includes(model)) : [];
  const valid = matching.length === 1 && matching[0].platform === 'grsai' &&
    matching[0].billing_mode === 'image' && matching[0].per_request_price === price;
  await check(name, valid, { ...ids, platform: 'grsai', model, price_usd: price, matches: valid });
}

async function readPublic(name, path, auth = 'user') {
  const { status, payload } = await request(path, { auth });
  const valid = status === 200 && payload?.code === 0;
  await check(name + '_read', valid, { http_status: status });
  requireValue(valid, 'PUBLIC_CONFIG_READ_FAILED');
  const serialized = JSON.stringify(payload.data).toLowerCase();
  const hidden = !serialized.includes('grsai') && !serialized.includes('grs.ai') &&
    !serialized.includes('"base_url"') && !serialized.includes('"credentials"');
  await check(name + '_provider_hidden', hidden, { provider_hidden: hidden });
  return payload.data;
}

async function main() {
  const env = await loadEnvironment();
  for (const [name, value] of Object.entries(env)) {
    if (/(?:^|_)(?:KEY|TOKEN|PASSWORD|SECRET)$/.test(name) && value) secrets.add(value);
  }
  adminKey = env.SUB2API_ADMIN_KEY;
  testKey = env.SUB2API_TEST_KEY;
  requireValue(adminKey && testKey && env.UPSTREAM_API_KEY && env.UPSTREAM_BASE_URL, 'MISSING_TEST_ENV');
  try {
    baseURL = new URL(env.SUB2API_TEST_BASE_URL);
  } catch {
    throw new CheckFailure('INVALID_LOCAL_TARGET');
  }
  requireValue(baseURL.protocol === 'http:' &&
    ['127.0.0.1', 'localhost', 'host.docker.internal'].includes(baseURL.hostname) &&
    baseURL.port === '18081' && baseURL.pathname === '/' &&
    !baseURL.username && !baseURL.password && !baseURL.search && !baseURL.hash, 'INVALID_LOCAL_TARGET');
  await check('local_preview_target', true);

  let fixture;
  try {
    fixture = JSON.parse(await readFile(new URL('../.dev/media-preview/test-fixture.json', import.meta.url), 'utf8'));
  } catch {
    throw new CheckFailure('FIXTURE_READ_FAILED');
  }
  const idFields = ['account_id', 'group_id', 'channel_id', 'user_id', 'api_key_id'];
  requireValue(isObject(fixture) && idFields.every(field => positiveID(fixture[field])) &&
    typeof fixture.model === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,127}$/.test(fixture.model) &&
    fixture.unit_price_usd === 0.01 &&
    (!env.SUB2API_TEST_MODEL || fixture.model === env.SUB2API_TEST_MODEL) &&
    (!env.SUB2API_TEST_USER_ID || fixture.user_id === Number(env.SUB2API_TEST_USER_ID)) &&
    (!env.SUB2API_TEST_API_KEY_ID || fixture.api_key_id === Number(env.SUB2API_TEST_API_KEY_ID)) &&
    (!env.SUB2API_TEST_EXPECTED_COST || fixture.unit_price_usd === Number(env.SUB2API_TEST_EXPECTED_COST)),
    'INVALID_TEST_FIXTURE');
  for (const field of idFields) report[field] = fixture[field];
  report.model = fixture.model;
  report.price_usd = fixture.unit_price_usd;
  const { account_id, group_id, channel_id, user_id, api_key_id, model, unit_price_usd: price } = fixture;
  const accountPath = `/api/v1/admin/accounts/${account_id}`;
  const groupPath = `/api/v1/admin/groups/${group_id}`;
  const channelPath = `/api/v1/admin/channels/${channel_id}`;
  const userPath = `/api/v1/admin/users/${user_id}`;
  const keysPath = `${userPath}/api-keys?page_size=100`;
  const resultPath = `/v1/api/result?id=media_${randomUUID()}`;
  allowedRequests = new Set([
    ...[accountPath, groupPath, channelPath, userPath, keysPath, resultPath].map(path => `GET ${path}`),
    'POST /api/v1/auth/login',
    ...['/groups/available', '/keys?page_size=100', `/keys/${api_key_id}`, '/channels/available',
      '/model-plaza', '/user/profile', '/user/platform-quotas', '/usage/dashboard/snapshot-v2',
      '/usage?exact_total=true&page_size=100'].map(path => `GET /api/v1${path}`),
  ]);

  const account = await readAdmin('account_read', accountPath, account_id, 'account_id');
  await check('account_grsai_platform', account.platform === 'grsai', { account_id, platform: 'grsai', matches: account.platform === 'grsai' });
  await check('account_api_key_type', account.type === 'apikey', { account_id, matches: account.type === 'apikey' });
  const upstreamURL = configuredURL(account.credentials?.base_url);
  const basePresent = typeof account.credentials?.base_url === 'string' && account.credentials.base_url.length > 0;
  const baseMatches = Boolean(upstreamURL && normalizedURL(account.credentials.base_url) === normalizedURL(env.UPSTREAM_BASE_URL));
  const custom = Boolean(upstreamURL && upstreamURL.hostname !== 'api.openai.com');
  await check('account_custom_base_url', basePresent && Boolean(upstreamURL) && custom && baseMatches, {
    account_id, present: basePresent, valid: Boolean(upstreamURL), custom, matches: baseMatches,
  });
  await check('account_upstream_key_configured', account.credentials_status?.has_api_key === true, {
    account_id, present: account.credentials_status?.has_api_key === true,
  });
  await check('account_upstream_key_redacted', !Object.hasOwn(account.credentials ?? {}, 'api_key'), {
    account_id, redacted: !Object.hasOwn(account.credentials ?? {}, 'api_key'),
  });
  const accountBound = Array.isArray(account.group_ids) && account.group_ids.includes(group_id);
  await check('account_group_binding', accountBound, { account_id, group_id, bound: accountBound });

  const group = await readAdmin('group_read', groupPath, group_id, 'group_id');
  await check('group_grsai_platform', group.platform === 'grsai', { group_id, platform: 'grsai', matches: group.platform === 'grsai' });
  await check('group_image_generation_enabled', group.allow_image_generation === true, { group_id, enabled: group.allow_image_generation === true });
  await check('group_standard_balance_billing', group.subscription_type === 'standard' && group.is_exclusive === false && group.rate_multiplier === 1, {
    group_id, standard: group.subscription_type === 'standard', exclusive: group.is_exclusive === true,
    matches: group.rate_multiplier === 1,
  });
  await checkPricing('group_explicit_image_price', group.model_pricing, model, price, { group_id });

  const channel = await readAdmin('channel_read', channelPath, channel_id, 'channel_id');
  const channelBound = Array.isArray(channel.group_ids) && channel.group_ids.includes(group_id);
  await check('channel_group_binding', channelBound, { channel_id, group_id, bound: channelBound });
  await checkPricing('channel_grsai_image_price', channel.model_pricing, model, price, { channel_id });
  await check('channel_requested_billing_model', channel.billing_model_source === 'requested', { channel_id, requested: channel.billing_model_source === 'requested' });
  await check('channel_models_restricted', channel.restrict_models === true, { channel_id, restricted: channel.restrict_models === true });

  const user = await readAdmin('user_read', userPath, user_id, 'user_id');
  const userBound = Array.isArray(user.allowed_groups) && user.allowed_groups.includes(group_id);
  await check('user_group_allowed', userBound, { user_id, group_id, bound: userBound });
  const { status: keysStatus, payload: keysPayload } = await request(keysPath);
  const keysValid = keysStatus === 200 && keysPayload?.code === 0 && Array.isArray(keysPayload?.data?.items);
  await check('user_api_keys_read', keysValid, { user_id, http_status: keysStatus });
  requireValue(keysValid, 'USER_API_KEYS_READ_FAILED');
  const key = keysPayload.data.items.find(item => item.id === api_key_id);
  await check('test_key_exists', Boolean(key), { user_id, api_key_id, present: Boolean(key) });
  await check('test_key_user_binding', key?.user_id === user_id, { user_id, api_key_id, bound: key?.user_id === user_id });
  await check('test_key_group_binding', key?.group_id === group_id, { api_key_id, group_id, bound: key?.group_id === group_id });
  await check('test_key_matches_fixture', key?.key === testKey, { api_key_id, matches: key?.key === testKey });
  await check('test_key_active', key?.status === 'active', { api_key_id, active: key?.status === 'active' });
  await check('admin_key_group_keeps_grsai_platform', key?.group?.platform === 'grsai', { api_key_id, platform: 'grsai' });

  const login = await request('/api/v1/auth/login', { method: 'POST', auth: 'none', body: {
    email: env.SUB2API_TEST_EMAIL, password: env.SUB2API_TEST_PASSWORD,
  } });
  requireValue(login.status === 200 && login.payload?.data?.access_token, 'TEST_USER_LOGIN_FAILED');
  userToken = login.payload.data.access_token;
  secrets.add(userToken);
  if (login.payload.data.refresh_token) secrets.add(login.payload.data.refresh_token);
  const publicGroups = await readPublic('public_groups', '/api/v1/groups/available');
  await check('public_group_media_platform', publicGroups.some(item => item.id === group_id && item.platform === 'media'), { group_id, platform: 'media' });
  const publicKeys = await readPublic('public_keys', '/api/v1/keys?page_size=100');
  await check('public_key_group_media_platform', publicKeys.items?.some(item => item.id === api_key_id && item.group?.platform === 'media'), { api_key_id, platform: 'media' });
  const publicKey = await readPublic('public_key_detail', `/api/v1/keys/${api_key_id}`);
  await check('public_key_detail_media_platform', publicKey.group?.platform === 'media', { api_key_id, platform: 'media' });
  const publicChannels = await readPublic('public_channels', '/api/v1/channels/available');
  const mediaSection = publicChannels.flatMap(item => item.platforms ?? []).find(item =>
    item.platform === 'media' && item.groups?.some(group => group.id === group_id));
  await check('public_channel_keeps_media_model_price', mediaSection?.supported_models?.some(item =>
    item.name === model && item.platform === 'media' && item.pricing?.billing_mode === 'image' &&
    item.pricing?.per_request_price === price), { group_id, platform: 'media', model, price_usd: price });
  const plaza = await readPublic('public_model_plaza', '/api/v1/model-plaza');
  await check('public_plaza_keeps_media_model', plaza.groups?.some(item => item.id === group_id &&
    item.platform === 'media' && item.models?.some(entry => entry.name === model && entry.platform === 'media')),
  { group_id, platform: 'media', model });
  await readPublic('anonymous_model_plaza', '/api/v1/model-plaza', 'none');
  await readPublic('public_profile', '/api/v1/user/profile');
  await readPublic('public_platform_quotas', '/api/v1/user/platform-quotas');
  await readPublic('public_dashboard', '/api/v1/usage/dashboard/snapshot-v2');
  await readPublic('public_usage', '/api/v1/usage?exact_total=true&page_size=100');

  const unauthenticated = await request(resultPath, { auth: 'none' });
  await check('result_requires_authentication', unauthenticated.status === 401, { http_status: unauthenticated.status, expected_http_status: 401 });
  const unknownTask = await request(resultPath, { auth: 'key' });
  const noTaskData = isObject(unknownTask.payload) && Object.keys(unknownTask.payload).every(field => field === 'error') &&
    isObject(unknownTask.payload.error) && unknownTask.payload.error.type === 'not_found_error' &&
    Object.keys(unknownTask.payload.error).every(field => ['type', 'message'].includes(field));
  await check('unknown_owned_task_not_found', unknownTask.status === 404 && noTaskData, {
    http_status: unknownTask.status, expected_http_status: 404, no_task_data: noTaskData,
  });

  report.passed = report.checks.every(item => item.passed);
  await persist();
  console.log(JSON.stringify({ passed: report.passed, checks: report.checks.length }));
  if (!report.passed) process.exitCode = 1;
}

try {
  await main();
} catch (error) {
  report.passed = false;
  // Never log raw API payloads, URLs, headers, credentials, or exception messages.
  report.result = error instanceof CheckFailure ? error.reason : 'CONFIG_CHECK_FAILED';
  try {
    await persist();
  } catch {
    report.result = 'REPORT_WRITE_FAILED';
  }
  console.error(JSON.stringify(scrub({ passed: false, result: report.result })));
  process.exitCode = 1;
}
