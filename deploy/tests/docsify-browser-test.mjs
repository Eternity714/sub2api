#!/usr/bin/env node
// Run the real browser through the docsify-browser-test Compose service.
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { randomUUID } from 'node:crypto'
import { writeFile, unlink } from 'node:fs/promises'

const require = createRequire('/opt/docsify-browser/package.json')
const { chromium } = require('playwright')
const baseURL = (process.env.DOCSIFY_TEST_URL || 'http://host.docker.internal:18083').replace(/\/$/, '')
const contentDir = process.env.DOCSIFY_TEST_CONTENT_DIR || '/workspace/frontend/docs'
const origin = new URL(baseURL).origin
const remoteFixture = process.env.DOCSIFY_TEST_REMOTE_FIXTURE
const routeCookie = process.env.DOCSIFY_TEST_COOKIE
const timeoutMs = Number(process.env.DOCSIFY_TEST_TIMEOUT_MS || 15_000)
assert.ok(Number.isInteger(timeoutMs) && timeoutMs > 0,
  'DOCSIFY_TEST_TIMEOUT_MS must be a positive integer')
if (remoteFixture) {
  assert.ok(remoteFixture.startsWith('/docs/') && !/[?#]/.test(remoteFixture)
    && !remoteFixture.endsWith('/') && !remoteFixture.endsWith('.md'),
  'DOCSIFY_TEST_REMOTE_FIXTURE must be an article path such as /docs/browser-live-update')
}
const externalRequests = []
const failedResponses = []
const pageErrors = []
const markdownRequests = new Set()

const browser = await chromium.launch({
  headless: true,
  // Full Chromium honors the secure-origin flag; the headless shell does not.
  channel: 'chromium',
  args: [`--unsafely-treat-insecure-origin-as-secure=${origin}`],
})

async function observe(context) {
  if (routeCookie) {
    await context.addCookies([{
      name: 'sub2api_docsify',
      value: routeCookie,
      domain: new URL(origin).hostname,
      path: '/docs',
      secure: new URL(origin).protocol === 'https:',
      sameSite: 'Lax',
    }])
  }
  // An unexpected external dependency is a failure even when its CDN is reachable.
  await context.route('**/*', async route => {
    const requestURL = new URL(route.request().url())
    if (['http:', 'https:'].includes(requestURL.protocol) && requestURL.origin !== origin) {
      externalRequests.push(requestURL.href)
      await route.abort()
    } else {
      await route.continue()
    }
  })
  context.on('request', request => {
    const requestURL = new URL(request.url())
    if (requestURL.pathname.endsWith('.md')) markdownRequests.add(requestURL.pathname)
  })
  context.on('response', response => {
    if (response.status() >= 400) failedResponses.push(`${response.status()} ${response.url()}`)
  })
  context.on('page', page => page.on('pageerror', error => pageErrors.push(error.message)))
}

async function openArticle(page, path, heading) {
  const response = await page.goto(baseURL + path, { waitUntil: 'networkidle' })
  assert.equal(response.status(), 200, `direct article ${path}`)
  await page.getByRole('heading', { level: 1, name: heading, exact: true }).waitFor()
  assert.equal(new URL(page.url()).pathname, path, 'history route changed the /docs prefix')
}

async function waitForMarkdownText(page, text) {
  await page.locator('.markdown-section').waitFor({ state: 'visible' })
  await page.waitForFunction(expected =>
    document.querySelector('.markdown-section')?.innerText.includes(expected), text)
}

async function waitForAsyncCondition(check, { timeout = timeoutMs, polling = 1000, description }) {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    const budget = Math.max(1, deadline - Date.now())
    let timer
    let ready
    try {
      ready = await Promise.race([
        check(budget),
        new Promise((_, reject) => {
          timer = setTimeout(() => reject(new Error(`${description} did not become ready within ${timeout}ms`)), budget)
        }),
      ])
    } finally {
      clearTimeout(timer)
    }
    if (ready && Date.now() < deadline) return
    const remaining = deadline - Date.now()
    if (remaining > 0) {
      await new Promise(resolve => setTimeout(resolve, Math.min(polling, remaining)))
    }
  }
  assert.fail(`${description} did not become ready within ${timeout}ms`)
}

async function step(name, action) {
  await action()
  console.log(`PASS ${name}`)
}

console.log(`Docsify browser target: ${baseURL}`)
let fixture
try {
  const desktop = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
    permissions: ['clipboard-read', 'clipboard-write'],
  })
  await observe(desktop)
  const page = await desktop.newPage()
  page.setDefaultTimeout(timeoutMs)
  page.setDefaultNavigationTimeout(timeoutMs)

  await step('async false readiness keeps polling until its deadline', async () => {
    let polls = 0
    await assert.rejects(waitForAsyncCondition(async () => {
      polls += 1
      return page.evaluate(async () => false)
    }, { timeout: 250, polling: 50, description: 'async false readiness' }),
    /async false readiness did not become ready within 250ms/)
    assert.ok(polls >= 2, 'an unresolved async condition stopped after its first false result')
    await assert.rejects(waitForAsyncCondition(() => new Promise(() => {}),
      { timeout: 100, description: 'pending async readiness' }),
    /pending async readiness did not become ready within 100ms/)
  })

  await step('home, sidebar and navbar render', async () => {
    const response = await page.goto(baseURL + '/docs/', { waitUntil: 'networkidle' })
    assert.equal(response.status(), 200)
    await page.locator('.markdown-section').getByText('推荐阅读顺序', { exact: true }).waitFor()
    await page.locator('.sidebar-nav a[href^="/docs/"]').nth(34).waitFor({ state: 'visible' })
    assert.ok(await page.locator('.sidebar-nav a[href^="/docs/"]').count() >= 35,
      'the existing tutorial sidebar is missing')
    await page.locator('.app-nav').getByRole('link', { name: '主站', exact: true }).waitFor()
    await page.locator('.app-nav').getByRole('link', { name: '控制台', exact: true }).waitFor()
    assert.equal(new URL(page.url()).pathname, '/docs/')
  })

  await step('sidebar navigation keeps the /docs path', async () => {
    await page.locator('.sidebar-nav').getByRole('link', { name: '快速开始', exact: true }).click()
    await page.getByRole('heading', { level: 1, name: '快速开始', exact: true }).waitFor()
    assert.equal(new URL(page.url()).pathname, '/docs/quick-start')
    assert.equal(new URL(page.url()).hash, '', 'article routes must use history mode')
    await waitForMarkdownText(page, 'Base URL 是接口地址')
    assert.ok((await page.locator('.markdown-section').innerText()).includes('Base URL 是接口地址'))
  })

  await step('code copy places the actual sample on the clipboard', async () => {
    const code = page.locator('.markdown-section pre').filter({ has: page.locator('code') }).first()
    const expected = (await code.locator('code').innerText()).trim()
    assert.ok(expected.includes('GKOTTA_API_KEY'), 'copy target is not the tutorial sample')
    await code.locator('.docsify-copy-code-button').click()
    await waitForAsyncCondition(() =>
      page.evaluate(expected => navigator.clipboard.readText().then(text => text.trim() === expected), expected),
    { polling: 100, description: 'copied tutorial sample to match the actual clipboard' })
    assert.equal((await page.evaluate(() => navigator.clipboard.readText())).trim(), expected)
  })

  await step('search indexes known article body text', async () => {
    await page.locator('.search input').fill('自动追加')
    const result = page.locator('.search .matching-post a[href*="/docs/quick-start"]').first()
    await result.waitFor({ state: 'visible' })
    assert.ok((await result.innerText()).includes('自动追加'), 'search found a title but missed the known body')
    await result.click()
    await page.getByRole('heading', { level: 1, name: '快速开始', exact: true }).waitFor()
    assert.equal(new URL(page.url()).pathname, '/docs/quick-start')
    await page.locator('.search input').fill('')
  })

  await step('directory and nested routes survive a full refresh', async () => {
    await openArticle(page, '/docs/scenarios/', '使用场景总览')
    await page.reload({ waitUntil: 'networkidle' })
    await page.getByRole('heading', { level: 1, name: '使用场景总览', exact: true }).waitFor()
    await openArticle(page, '/docs/scenarios/cherry-studio', 'Cherry Studio')
    await page.reload({ waitUntil: 'networkidle' })
    await page.getByRole('heading', { level: 1, name: 'Cherry Studio', exact: true }).waitFor()
    assert.equal(new URL(page.url()).pathname, '/docs/scenarios/cherry-studio')
  })

  await step('Mermaid produces a visible SVG with the actual flow labels', async () => {
    await page.goto(baseURL + '/docs/scenarios/lark-images', { waitUntil: 'networkidle' })
    const diagram = page.locator('.markdown-section .mermaid svg').first()
    await diagram.waitFor({ state: 'visible' })
    assert.ok((await diagram.textContent()).includes('Gkotta 图片 API'), 'Mermaid did not render the tutorial diagram')
    const bounds = await diagram.boundingBox()
    assert.ok(bounds.width > 0 && bounds.height > 0, 'Mermaid SVG has no visible dimensions')
    assert.equal(await page.locator('.markdown-section code.lang-mermaid').count(), 0,
      'the diagram remained an unrendered code block')
  })

  await step('mounted Markdown changes appear after browser refresh', async () => {
    let articlePath = remoteFixture
    if (!articlePath) {
      const name = `docsify-browser-live-${randomUUID()}`
      fixture = `${contentDir}/${name}.md`
      articlePath = `/docs/${name}`
      await writeFile(fixture, '# Browser live update\n\nInitial mounted browser content.\n')
    }
    await openArticle(page, articlePath, 'Browser live update')
    await waitForMarkdownText(page, 'Initial mounted browser content.')
    assert.ok((await page.locator('.markdown-section').innerText()).includes('Initial mounted browser content.'))
    if (remoteFixture) {
      console.log('READY remote fixture mounted initial content')
      await waitForAsyncCondition(remaining => page.evaluate(async ({ path, remaining }) => {
        const response = await fetch(path + '.md', { cache: 'no-store', signal: AbortSignal.timeout(remaining) })
        return response.ok && (await response.text()).includes('Revised mounted browser content.')
      }, { path: articlePath, remaining }),
      { timeout: 60_000, description: 'remote fixture to contain Revised mounted browser content.' })
    } else {
      await writeFile(fixture, '# Browser live update\n\nRevised mounted browser content.\n')
    }
    await page.reload({ waitUntil: 'networkidle' })
    await waitForMarkdownText(page, 'Revised mounted browser content.')
    const content = await page.locator('.markdown-section').innerText()
    assert.ok(content.includes('Revised mounted browser content.'))
    assert.ok(!content.includes('Initial mounted browser content.'))
    if (fixture) {
      await unlink(fixture)
      fixture = undefined
    }
  })

  await step('mobile articles contain horizontal scrolling within content blocks', async () => {
    const mobile = await browser.newContext({
      viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true,
    })
    await observe(mobile)
    const mobilePage = await mobile.newPage()
    mobilePage.setDefaultTimeout(timeoutMs)
    mobilePage.setDefaultNavigationTimeout(timeoutMs)
    await mobilePage.goto(baseURL + '/docs/', { waitUntil: 'networkidle' })
    await waitForMarkdownText(mobilePage, '推荐阅读顺序')
    await mobilePage.locator('.sidebar-toggle').click()
    await mobilePage.locator('.sidebar-nav').getByRole('link', { name: '快速开始', exact: true }).click()
    await mobilePage.getByRole('heading', { level: 1, name: '快速开始', exact: true }).waitFor()
    assert.equal(new URL(mobilePage.url()).pathname, '/docs/quick-start', 'mobile navigation failed')
    for (const [path, expectedText] of [
      ['/docs/', '推荐阅读顺序'],
      ['/docs/quick-start', 'Base URL 是接口地址'],
      ['/docs/scenarios/lark-images', '飞书多维表格图片生成'],
    ]) {
      await mobilePage.goto(baseURL + path, { waitUntil: 'networkidle' })
      await waitForMarkdownText(mobilePage, expectedText)
      if (path === '/docs/scenarios/lark-images') {
        await mobilePage.locator('.markdown-section .mermaid svg').first().waitFor({ state: 'visible' })
      }
      const dimensions = await mobilePage.evaluate(() => ({
        viewport: document.documentElement.clientWidth,
        document: document.documentElement.scrollWidth,
        body: document.body.scrollWidth,
      }))
      assert.ok(dimensions.document <= dimensions.viewport + 2,
        `${path} overflows the mobile document: ${JSON.stringify(dimensions)}`)
      assert.ok(dimensions.body <= dimensions.viewport + 2,
        `${path} overflows the mobile body: ${JSON.stringify(dimensions)}`)
    }
    await mobile.close()
  })

  await step('every raw Markdown request retains /docs and all runtime requests stay local', async () => {
    for (const path of ['/docs/index.md', '/docs/quick-start.md', '/docs/scenarios/index.md',
      '/docs/scenarios/cherry-studio.md', '/docs/scenarios/lark-images.md']) {
      assert.ok(markdownRequests.has(path), `browser did not fetch ${path}`)
    }
    for (const path of markdownRequests) assert.ok(path.startsWith('/docs/'), `lost Markdown prefix: ${path}`)
    assert.deepEqual(externalRequests, [], 'the document runtime attempted external requests')
    assert.deepEqual(failedResponses, [], 'a document or runtime resource failed to load')
    assert.deepEqual(pageErrors, [], 'browser runtime errors')
  })
  await desktop.close()
  console.log('PASS all Docsify browser regression checks')
} finally {
  if (fixture) await unlink(fixture).catch(() => {})
  await browser.close()
}
