import { readFile, writeFile } from 'node:fs/promises';

const base = process.env.SUB2API_TEST_BASE_URL || 'http://host.docker.internal:18081';
const adminKey = process.env.SUB2API_ADMIN_KEY;
const userKey = process.env.SUB2API_TEST_KEY;
const userId = Number(process.env.SUB2API_TEST_USER_ID);
const keyId = Number(process.env.SUB2API_TEST_API_KEY_ID);
const expectedCost = Number(process.env.SUB2API_TEST_EXPECTED_COST || '0.01');
const initialBalance = Number(process.env.SUB2API_TEST_INITIAL_BALANCE || '10');
const expectedRequests = Number(process.env.SUB2API_TEST_EXPECTED_REQUESTS || '1');
function assert(value, message) { if (!value) throw new Error(message); }
const near = (a, b) => Math.abs(a - b) < 0.0000001;
assert(adminKey && userKey && userId > 0 && keyId > 0, 'Billing audit environment is incomplete');
assert(Number.isFinite(expectedCost) && expectedCost > 0 && Number.isFinite(initialBalance), 'Billing expectations are invalid');
assert(Number.isInteger(expectedRequests) && expectedRequests > 0, 'Expected request count must be positive');

async function get(path, admin = true) {
  const response = await fetch(base + path, {
    headers: admin ? { 'x-api-key': adminKey } : { Authorization: 'Bearer ' + userKey },
    signal: AbortSignal.timeout(30000), redirect: 'error',
  });
  assert(response.ok, `${path.split('?')[0]} returned HTTP ${response.status}`);
  const json = await response.json();
  assert(json.code === undefined || json.code === 0, 'API envelope reports a failure');
  return json.data ?? json;
}
async function billing() {
  const [user, logs] = await Promise.all([
    get('/api/v1/admin/users/' + userId),
    get('/api/v1/admin/usage?api_key_id=' + keyId + '&exact_total=true&page_size=100'),
  ]);
  return { balance: Number(user.balance), usageTotal: logs.total, rows: logs.items.map(row => ({
    id: row.id, model: row.model, api_key_id: row.api_key_id, actual_cost: Number(row.actual_cost),
    total_cost: Number(row.total_cost), image_count: row.image_count, billing_mode: row.billing_mode,
  })) };
}

try {
  const checkpoint = JSON.parse(await readFile(new URL('./results/last-run.json', import.meta.url), 'utf8'));
  assert(checkpoint.passed && checkpoint.status === 'succeeded', 'Run the image test successfully before checking billing');
  assert(/^(?:media|grsai)_[0-9a-f-]{36}$/i.test(checkpoint.taskId), 'Checkpoint has no valid local task ID');
  let before;
  for (let attempt = 0; attempt < 12; attempt++) {
    before = await billing();
    if (before.usageTotal === expectedRequests) break;
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
  assert(before.usageTotal === expectedRequests && before.rows.length === expectedRequests, 'Expected exactly one usage record per successful task');
  assert(near(before.balance, initialBalance - expectedCost * expectedRequests), 'Balance does not match the expected image charge');
  assert(before.rows.every(row => row.model === checkpoint.model && row.api_key_id === keyId && row.image_count === 1 && row.billing_mode === 'image' && near(row.actual_cost, expectedCost)), 'Usage pricing or image count does not match the configured test');

  // Query the same task repeatedly; this never submits a new upstream request.
  for (let attempt = 0; attempt < 3; attempt++) {
    const task = await get('/v1/api/result?id=' + encodeURIComponent(checkpoint.taskId), false);
    assert(task.id === checkpoint.taskId && task.status === 'succeeded', 'Completed task changed during repeated queries');
  }
  const after = await billing();
  assert(near(after.balance, before.balance) && after.usageTotal === before.usageTotal && JSON.stringify(after.rows) === JSON.stringify(before.rows), 'Repeated queries changed billing');
  const report = { passed: true, at: new Date().toISOString(), taskId: checkpoint.taskId, model: checkpoint.model,
    initialBalance, expectedCost, expectedRequests, charged: initialBalance - after.balance,
    repeatQueries: 3, repeatedQueryCharge: Number((before.balance - after.balance).toFixed(8)), ...after };
  await writeFile(new URL('./results/billing-report.json', import.meta.url), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  // Never print API responses, URLs, authentication headers, or raw network exceptions.
  console.error('Billing audit failed: ' + (error.message?.includes('http') ? 'request or response error' : error.message));
  process.exitCode = 1;
}
