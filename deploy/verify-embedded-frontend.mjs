#!/usr/bin/env node
import { readFile, realpath, stat } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// Verify the tree that Go embeds, including Vite's lazy route dependencies.
// Resolve asset references under dist; never evaluate the generated JavaScript.
const defaultDist = fileURLToPath(new URL('../backend/internal/web/dist/', import.meta.url))
const dist = path.resolve(process.argv[2] ?? defaultDist)
const missing = new Set()
const invalid = new Set()
const checked = new Set()

function isInside(root, candidate) {
  const relative = path.relative(root, candidate)
  return relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative)
}

function assetPath(reference) {
  let decoded
  try {
    decoded = decodeURIComponent(reference.split(/[?#]/, 1)[0])
  } catch {
    invalid.add(reference)
    return null
  }
  const normalized = decoded.replace(/^\.?\//, '')
  if (!normalized.startsWith('assets/') || normalized.includes('\\') || normalized.includes('\0') ||
      normalized.split('/').some(segment => segment === '..' || segment === '.')) {
    invalid.add(reference)
    return null
  }
  return normalized
}

try {
  if (process.argv.length > 3) throw new Error('usage: node verify-embedded-frontend.mjs [dist-directory]')
  const root = await realpath(dist)
  const indexPath = path.join(root, 'index.html')
  if (!isInside(root, await realpath(indexPath))) throw new Error('index.html escapes dist')
  const html = await readFile(indexPath, 'utf8')
  const pending = new Set()
  const entryScripts = new Set()
  for (const match of html.matchAll(/\b(?:src|href)\s*=\s*["'](\/assets\/[^"']+\.(?:m?js|css)(?:[?#][^"']*)?)["']/gi)) {
    const asset = assetPath(match[1])
    if (asset) pending.add(asset)
  }
  for (const match of html.matchAll(/<script\b[^>]*\bsrc\s*=\s*["'](\/assets\/[^"']+\.m?js(?:[?#][^"']*)?)["']/gi)) {
    const asset = assetPath(match[1])
    if (asset) entryScripts.add(asset)
  }
  if (pending.size === 0) throw new Error('index.html has no /assets/ JavaScript or CSS references')
  if (entryScripts.size === 0) throw new Error('index.html has no /assets/ JavaScript entry point')
  for (const asset of pending) {
    if (checked.has(asset)) continue
    checked.add(asset)
    const candidate = path.resolve(root, asset)
    if (!isInside(root, candidate)) {
      invalid.add(asset)
      continue
    }
    let canonical
    try {
      canonical = await realpath(candidate)
      if (!(await stat(canonical)).isFile()) {
        missing.add(asset)
        continue
      }
    } catch (error) {
      if (error.code === 'ENOENT' || error.code === 'ENOTDIR') {
        missing.add(asset)
        continue
      }
      throw error
    }
    if (!isInside(root, canonical)) {
      invalid.add(asset)
      continue
    }
    if (entryScripts.has(asset)) {
      const source = await readFile(canonical, 'utf8')
      for (const match of source.matchAll(/(["'`])((?:\/|\.\/)?assets\/[^"'`\s]+?\.(?:m?js|css)(?:[?#][^"'`\s]*)?)\1/g)) {
        const dependency = assetPath(match[2])
        if (dependency) pending.add(dependency)
      }
    }
  }
  const ok = missing.size === 0 && invalid.size === 0
  console.log(JSON.stringify({ ok, dist, checked_assets: checked.size, missing: [...missing].sort(), invalid: [...invalid].sort() }))
  if (!ok) process.exitCode = 1
} catch (error) {
  console.log(JSON.stringify({ ok: false, dist, error: error.message, missing: [...missing].sort(), invalid: [...invalid].sort() }))
  process.exitCode = 1
}
