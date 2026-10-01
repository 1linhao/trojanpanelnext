import { readdir, readFile, stat } from 'node:fs/promises';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..');
const excluded = new Set(['.git', '.local', '.artifacts', 'node_modules', 'dist']);
const errors = [];
const anchors = new Map();

async function files(directory) {
  const found = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.isDirectory() && excluded.has(entry.name)) continue;
    const target = path.join(directory, entry.name);
    if (target === path.join(root, 'apps/docs-site/docs')) continue;
    if (entry.isDirectory()) found.push(...await files(target));
    if (entry.isFile() && entry.name.endsWith('.md')) found.push(target);
  }
  return found;
}

function headings(markdown) {
  const found = new Set();
  const duplicates = new Map();
  for (const match of markdown.matchAll(/\bid=["']([^"']+)["']/g)) found.add(match[1]);
  for (const match of markdown.matchAll(/^#{1,6}\s+(.+)$/gm)) {
    const slug = match[1].replace(/<[^>]+>/g, '').replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
      .toLowerCase().replace(/[^\p{L}\p{N}_\s-]/gu, '').replace(/\s/g, '-');
    const count = duplicates.get(slug) ?? 0;
    found.add(count ? `${slug}-${count}` : slug);
    duplicates.set(slug, count + 1);
  }
  return found;
}

for (const file of await files(root)) {
  const markdown = await readFile(file, 'utf8');
  const text = markdown.replace(/^```[^\n]*\n[\s\S]*?^```\s*$/gm, '');
  for (const match of text.matchAll(/!?\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g)) {
    const href = match[1];
    if (/^(?:[a-z][a-z0-9+.-]*:|\/)/i.test(href)) continue;
    const [name, fragment] = href.split('#');
    const target = name ? path.resolve(path.dirname(file), decodeURIComponent(name)) : file;
    const relative = path.relative(root, file);
    try {
      await stat(target);
      if (fragment && target.endsWith('.md')) {
        if (!anchors.has(target)) anchors.set(target, headings(await readFile(target, 'utf8')));
        if (!anchors.get(target).has(decodeURIComponent(fragment))) errors.push(`${relative}: missing title anchor ${href}`);
      }
    } catch {
      errors.push(`${relative}: missing local target ${href}`);
    }
  }
}
if (errors.length) {
  process.stderr.write(`${errors.join('\n')}\n`);
  process.exit(1);
}
console.log('PASS documentation local links and title anchors');
