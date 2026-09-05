import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'

const root = path.resolve(import.meta.dirname, '..')
const ignoredDirectories = new Set(['.git', '.local', 'dist', 'node_modules'])
const failures = []

async function collectReadmes(directory) {
  const results = []
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.isDirectory() && ignoredDirectories.has(entry.name)) continue
    const entryPath = path.join(directory, entry.name)
    if (entry.isDirectory()) results.push(...await collectReadmes(entryPath))
    if (entry.isFile() && entry.name === 'README.md') results.push(entryPath)
  }
  return results
}

function withoutFrontMatter(markdown) {
  if (!markdown.startsWith('---\n')) return markdown
  const end = markdown.indexOf('\n---\n', 4)
  return end === -1 ? markdown : markdown.slice(end + 5)
}

for (const chinesePath of await collectReadmes(root)) {
  const relative = path.relative(root, chinesePath)
  const englishPath = path.join(path.dirname(chinesePath), 'README_EN.md')
  const chinese = await readFile(chinesePath, 'utf8')
  let english = ''
  try {
    english = await readFile(englishPath, 'utf8')
  } catch {
    failures.push(`${relative}: missing README_EN.md`)
    continue
  }

  if (!chinese.includes('简体中文') || !chinese.includes('[English](README_EN.md)')) {
    failures.push(`${relative}: missing Chinese-default language switch`)
  }
  if (!english.includes('[简体中文](README.md)') || !english.includes('English')) {
    failures.push(`${path.relative(root, englishPath)}: missing Chinese backlink`)
  }

  for (const [index, line] of withoutFrontMatter(chinese).split('\n').entries()) {
    if (/^\s{2,}[-*+]\s/.test(line)) failures.push(`${relative}:${index + 1}: nested Markdown list item`)
  }
  for (const [index, line] of withoutFrontMatter(english).split('\n').entries()) {
    if (/^\s{2,}[-*+]\s/.test(line)) failures.push(`${path.relative(root, englishPath)}:${index + 1}: nested Markdown list item`)
  }
}

if (failures.length) {
  process.stderr.write(`${failures.join('\n')}\n`)
  process.exit(1)
}

process.stdout.write('PASS every README defaults to Chinese, links to English, and avoids nested lists\n')
