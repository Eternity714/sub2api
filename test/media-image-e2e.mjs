import { createHash } from 'node:crypto';
import { mkdir, readFile, rename, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const resultsDirectory = fileURLToPath(new URL('./results/', import.meta.url));
const checkpointFile = new URL('./results/last-run.json', import.meta.url);
const pollLimitMilliseconds = 10 * 60 * 1000;
const pollIntervalMilliseconds = 5000;
const maximumImageBytes = 32 * 1024 * 1024;
const localTaskPattern = /^(?:media|grsai)_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const terminalStatuses = new Set(['succeeded', 'failed', 'manual_review']);
const allowedStatuses = new Set(['queued', 'running', ...terminalStatuses]);

let report;
let reportFile;
let baseURL;
let authorization;

class TestFailure extends Error {
  constructor(code, message, retryable = false) {
    super(message);
    this.code = code;
    this.retryable = retryable;
  }
}

function assert(condition, code, message) {
  if (!condition) throw new TestFailure(code, message);
}

function hash(value) {
  return createHash('sha256').update(value).digest('hex');
}

async function persist() {
  if (!report) return;
  const json = `${JSON.stringify(report, null, 2)}\n`;
  await writeFile(reportFile, json, { mode: 0o600 });
  const temporary = new URL('./results/last-run.json.tmp', import.meta.url);
  await writeFile(temporary, json, { mode: 0o600 });
  await rename(temporary, checkpointFile);
}

async function stage(name, details = {}) {
  const item = { at: new Date().toISOString(), stage: name, ...details };
  report.stages.push(item);
  await persist();
  console.log(JSON.stringify(item));
}

async function readCheckpoint() {
  let text;
  try {
    text = await readFile(checkpointFile, 'utf8');
  } catch (error) {
    if (error.code === 'ENOENT') return null;
    throw new TestFailure('CHECKPOINT_READ_FAILED', '无法读取恢复记录，已停止以避免重复提交。');
  }
  try {
    const previous = JSON.parse(text);
    assert(previous && previous.schemaVersion === 1, 'CHECKPOINT_INVALID', '恢复记录格式无效，已停止以避免重复提交。');
    assert(!previous.taskId || localTaskPattern.test(previous.taskId), 'CHECKPOINT_INVALID', '恢复记录任务 ID 无效，已停止以避免重复提交。');
    return previous;
  } catch (error) {
    if (error instanceof TestFailure) throw error;
    throw new TestFailure('CHECKPOINT_INVALID', '恢复记录不是有效 JSON，已停止以避免重复提交。');
  }
}

async function readLimitedBody(response, limit) {
  assert(response.body, 'EMPTY_RESPONSE', '响应没有内容。');
  const reader = response.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.byteLength;
      assert(size <= limit, 'RESPONSE_TOO_LARGE', '响应内容超过允许大小。');
      chunks.push(Buffer.from(value));
    }
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
  return Buffer.concat(chunks, size);
}

async function requestJSON(path, { method = 'GET', body, timeout = 30000, authenticated = true } = {}) {
  let response;
  try {
    response = await fetch(new URL(path, baseURL), {
      method,
      redirect: 'error',
      headers: {
        Accept: 'application/json',
        ...(authenticated ? { Authorization: authorization } : {}),
        ...(body ? { 'Content-Type': 'application/json' } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(Math.max(1, timeout)),
    });
    if (!response.ok) {
      await response.body?.cancel();
      throw new TestFailure(`HTTP_${response.status}`, `接口返回 HTTP ${response.status}。`, response.status === 429 || response.status >= 500);
    }
    const bytes = await readLimitedBody(response, 1024 * 1024);
    let payload;
    try {
      payload = JSON.parse(bytes.toString('utf8'));
    } catch {
      throw new TestFailure('INVALID_JSON', '接口未返回有效 JSON。');
    }
    return { status: response.status, payload };
  } catch (error) {
    if (error instanceof TestFailure) throw error;
    throw new TestFailure('NETWORK_REQUEST_FAILED', '接口请求失败或超时；原始响应与异常未输出。', true);
  }
}

function unwrapTask(payload) {
  // Current gateway returns a raw task object. Also accept the standard data envelope.
  const task = payload && typeof payload.id === 'string' ? payload : payload?.data;
  assert(task && typeof task === 'object' && !Array.isArray(task), 'INVALID_TASK', '接口没有返回任务对象。');
  assert(typeof task.id === 'string' && localTaskPattern.test(task.id), 'INVALID_TASK_ID', '接口没有返回有效的本地任务 ID。');
  assert(allowedStatuses.has(task.status), 'INVALID_TASK_STATUS', '接口返回了未知的任务状态。');
  return task;
}

function mediaURLs(task) {
  assert(task.link_expired !== true, 'MEDIA_LINK_EXPIRED', '任务媒体链接已过期。');
  const media = task.result?.results;
  assert(Array.isArray(media) && media.length > 0, 'MISSING_MEDIA', '成功任务没有 result.results 图片结果。');
  return media.map((item) => {
    assert(item && typeof item.url === 'string', 'INVALID_MEDIA_URL', '图片结果缺少 URL。');
    let url;
    try { url = new URL(item.url); } catch { throw new TestFailure('INVALID_MEDIA_URL', '图片 URL 格式无效。'); }
    assert(['http:', 'https:'].includes(url.protocol) && !url.username && !url.password, 'INVALID_MEDIA_URL', '图片 URL 必须为无内嵌凭据的 HTTP(S) 地址。');
    return url;
  });
}

async function pollTask(taskId) {
  const deadline = Date.now() + pollLimitMilliseconds;
  let lastStatus;
  let lastRetryCode;
  while (Date.now() < deadline) {
    let task;
    try {
      const { payload } = await requestJSON(`/v1/api/result?id=${encodeURIComponent(taskId)}`, {
        timeout: Math.min(30000, Math.max(1, deadline - Date.now())),
      });
      task = unwrapTask(payload);
      assert(task.id === taskId, 'TASK_ID_MISMATCH', '查询返回的任务 ID 与提交 ID 不一致。');
    } catch (error) {
      if (!(error instanceof TestFailure) || !error.retryable) throw error;
      if (error.code !== lastRetryCode) await stage('poll-retry', { code: error.code });
      lastRetryCode = error.code;
      await new Promise((resolve) => setTimeout(resolve, Math.max(0, Math.min(pollIntervalMilliseconds, deadline - Date.now()))));
      continue;
    }
    lastRetryCode = undefined;
    report.pollCount++;
    report.status = task.status;
    if (task.status !== lastStatus) {
      await stage('poll', { taskId, status: task.status });
      lastStatus = task.status;
    } else {
      await persist();
    }
    if (terminalStatuses.has(task.status)) {
      assert(task.status === 'succeeded', 'TASK_FAILED', `任务进入 ${task.status} 终态；请在本地后台查看错误详情。`);
      return task;
    }
    await new Promise((resolve) => setTimeout(resolve, Math.max(0, Math.min(pollIntervalMilliseconds, deadline - Date.now()))));
  }
  throw new TestFailure('POLL_TIMEOUT', '轮询已达 10 分钟，请用保存的任务 ID 恢复查询，勿再次提交。');
}

function imageFormat(bytes) {
  if (bytes.length >= 8 && bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))) return { extension: 'png', mime: 'image/png' };
  if (bytes.length >= 3 && bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255) return { extension: 'jpg', mime: 'image/jpeg' };
  if (bytes.length >= 12 && bytes.toString('ascii', 0, 4) === 'RIFF' && bytes.toString('ascii', 8, 12) === 'WEBP') return { extension: 'webp', mime: 'image/webp' };
  if (bytes.length >= 6 && ['GIF87a', 'GIF89a'].includes(bytes.toString('ascii', 0, 6))) return { extension: 'gif', mime: 'image/gif' };
  throw new TestFailure('INVALID_IMAGE_BYTES', '下载内容不是受支持的 PNG/JPEG/WebP/GIF 图片，可能为错误页。');
}

function downloadURL(originalURL) {
  const url = new URL(originalURL);
  const loopback = ['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname);
  if (baseURL.hostname === 'host.docker.internal' && url.protocol === 'http:' && loopback) {
    // Local public MinIO links need the host route inside Docker. Never rewrite signed links.
    const signed = [...url.searchParams.keys()].some((key) => /signature|credential|token/i.test(key));
    assert(!signed, 'LOOPBACK_SIGNED_URL', '容器无法直接访问带签名的 localhost 媒体链接，请配置容器可访问的存储 URL。');
    url.hostname = 'host.docker.internal';
  }
  return url;
}

async function downloadImage(url, index) {
  let response;
  let bytes;
  try {
    response = await fetch(downloadURL(url), { redirect: 'follow', signal: AbortSignal.timeout(60000) });
    assert(response.ok, 'MEDIA_HTTP_FAILED', `图片下载返回 HTTP ${response.status}。`);
    const contentType = (response.headers.get('content-type') || '').split(';')[0].trim().toLowerCase();
    assert(contentType.startsWith('image/'), 'INVALID_MEDIA_CONTENT_TYPE', '图片下载 Content-Type 不是 image/*，可能为 HTML 错误页。');
    bytes = await readLimitedBody(response, maximumImageBytes);
    assert(bytes.length > 0, 'EMPTY_MEDIA', '下载的图片为空。');
    const format = imageFormat(bytes);
    assert(contentType === format.mime || (format.mime === 'image/jpeg' && contentType === 'image/jpg'), 'IMAGE_TYPE_MISMATCH', '下载内容与图片 Content-Type 不一致。');
    const filename = `${report.taskId}-image-${index + 1}.${format.extension}`;
    await writeFile(new URL(`./results/${filename}`, import.meta.url), bytes, { mode: 0o600 });
    return { file: filename, bytes: bytes.length, contentType: format.mime, sha256: hash(bytes), sourceURLHash: hash(url.href) };
  } catch (error) {
    await response?.body?.cancel().catch(() => {});
    if (error instanceof TestFailure) throw error;
    throw new TestFailure('MEDIA_DOWNLOAD_FAILED', '图片下载或保存失败；原始 URL 与异常未输出。');
  }
}

async function main() {
  const key = (process.env.SUB2API_TEST_KEY || '').trim();
  const model = (process.env.SUB2API_TEST_MODEL || 'nano-banana-2-lite').trim();
  const requestedTaskId = (process.env.TASK_ID || '').trim();
  const newRun = process.argv.slice(2).includes('--new-run');
  assert(process.argv.slice(2).every((argument) => argument === '--new-run'), 'INVALID_ARGUMENT', '仅支持 --new-run 参数。');
  assert(!newRun || !requestedTaskId, 'CONFLICTING_ARGUMENTS', '--new-run 与 TASK_ID 不能同时使用。');
  assert(key, 'MISSING_TEST_KEY', '请在忽略的 media-test.env 配置 SUB2API_TEST_KEY。');
  assert(/^[a-z0-9][a-z0-9._:/-]{0,127}$/i.test(model), 'INVALID_MODEL', '测试模型名称无效。');
  assert(!requestedTaskId || localTaskPattern.test(requestedTaskId), 'INVALID_TASK_ID', 'TASK_ID 必须为本地 media_ 或 grsai_ UUID。');
  try { baseURL = new URL(process.env.SUB2API_TEST_BASE_URL || 'http://host.docker.internal:18081'); }
  catch { throw new TestFailure('INVALID_BASE_URL', 'SUB2API_TEST_BASE_URL 格式无效。'); }
  assert(['http:', 'https:'].includes(baseURL.protocol) && !baseURL.username && !baseURL.password && !baseURL.search && !baseURL.hash && ['/', ''].includes(baseURL.pathname), 'INVALID_BASE_URL', '测试 Base URL 需为无凭据、查询参数和路径的 HTTP(S) 服务根地址。');
  authorization = `Bearer ${key}`;
  await mkdir(resultsDirectory, { recursive: true });
  const previous = await readCheckpoint();
  const taskId = requestedTaskId || (!newRun && previous?.taskId) || null;
  const runId = `image-e2e-${new Date().toISOString().replace(/[:.]/g, '-')}`;
  reportFile = new URL(`./results/${runId}.json`, import.meta.url);
  report = {
    schemaVersion: 1, runId, startedAt: new Date().toISOString(), model, taskId,
    submissionAttempted: Boolean(taskId || (!newRun && previous?.submissionAttempted)),
    resumed: Boolean(taskId), pollCount: 0, status: null, images: [], stages: [],
  };
  assert(taskId || newRun || !previous?.submissionAttempted, 'SUBMISSION_UNKNOWN', '之前已尝试提交但未保存任务 ID；请在后台核查任务，脚本拒绝再次提交收费。');
  await stage('start', { resumed: Boolean(taskId), model });
  await requestJSON('/health', { authenticated: false });
  await stage('health', { ok: true });
  if (taskId) {
    await stage('resume', { taskId });
  } else {
    report.submissionAttempted = true;
    await stage('submit-attempt', { model });
    // There is exactly one submission call. A transport error never retries this POST.
    const { status, payload } = await requestJSON('/v1/api/generate', {
      method: 'POST',
      body: { model, prompt: 'A small orange kitten sitting on a white background.', aspectRatio: '1:1', imageSize: '1K', images: [], replyType: 'async' },
      timeout: 60000,
    });
    const task = unwrapTask(payload);
    report.taskId = task.id;
    report.status = task.status;
    await stage('submitted', { taskId: task.id, status: task.status, httpStatus: status });
    assert(status === 202, 'EXPECTED_ASYNC_ACCEPTED', '生成接口应返回 HTTP 202；任务 ID 已保存，请恢复查询。');
  }
  const completed = await pollTask(report.taskId);
  const urls = mediaURLs(completed);
  await stage('download-start', { imageCount: urls.length });
  for (let index = 0; index < urls.length; index++) {
    report.images.push(await downloadImage(urls[index], index));
    await stage('downloaded', { image: index + 1, bytes: report.images[index].bytes, contentType: report.images[index].contentType });
  }
  const repeated = unwrapTask((await requestJSON(`/v1/api/result?id=${encodeURIComponent(report.taskId)}`)).payload);
  assert(repeated.id === report.taskId && repeated.status === completed.status, 'REPEAT_STATUS_CHANGED', '重复查询的任务 ID 或状态不一致。');
  const repeatedURLs = mediaURLs(repeated);
  assert(JSON.stringify(repeatedURLs.map((url) => url.href)) === JSON.stringify(urls.map((url) => url.href)), 'REPEAT_MEDIA_CHANGED', '同一任务重复查询的图片 URL 不一致。');
  await stage('repeat-query', { ok: true, taskId: report.taskId });
  report.passed = true;
  report.finishedAt = new Date().toISOString();
  await stage('passed', { taskId: report.taskId, imageCount: report.images.length });
  console.log(`测试通过。报告：results/${runId}.json；恢复记录：results/last-run.json`);
}

main().catch(async (error) => {
  const failure = error instanceof TestFailure ? error : new TestFailure('UNEXPECTED_FAILURE', '测试发生异常；原始错误未输出，请检查容器和结果目录。');
  if (report) {
    report.passed = false;
    report.finishedAt = new Date().toISOString();
    report.failure = { code: failure.code, message: failure.message };
    try { await stage('failed', { code: failure.code, message: failure.message, taskId: report.taskId }); }
    catch { console.error('报告写入失败，请检查 results 目录权限；勿重复提交。'); }
  } else {
    console.error(JSON.stringify({ stage: 'failed', code: failure.code, message: failure.message }));
  }
  process.exitCode = 1;
});
