import { readdir, readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
const root = process.cwd();
const assets = [];
async function walk(directory, prefix) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const file = path.join(directory, entry.name);
    const url = `${prefix}/${entry.name}`;
    if (entry.isDirectory()) await walk(file, url);
    else if (!['/asset-manifest.json', '/asset-worker.js', '/indexpage/le0ou.jpg'].includes(url) && !entry.name.endsWith('.map') && !entry.name.startsWith('.')) {
      const buffer = await readFile(file);
      assets.push({ url, bytes: buffer.length, hash: createHash('sha256').update(buffer).digest('hex') });
    }
  }
}
await walk(path.join(root, 'public'), '');
if (!process.argv.includes('--dev')) await walk(path.join(root, process.env.NEXT_DIST_DIR || '.next', 'static'), '/_next/static');
assets.sort((a, b) => a.url.localeCompare(b.url));
const version = createHash('sha256').update(JSON.stringify(assets)).digest('hex').slice(0, 16);
await writeFile('public/asset-manifest.json', JSON.stringify({ version, assets }));
// Only immutable build files and explicitly enumerated public assets are cached.
// API requests, authenticated HTML and RSC responses never enter this cache.
await writeFile('public/asset-worker.js', `
const CACHE = 'home-assets-${version}';
const PATHS = new Set(${JSON.stringify(assets.map(a => a.url))});
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', event => event.waitUntil(self.clients.claim()));
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);
  if (event.request.method !== 'GET' || url.origin !== self.location.origin || !PATHS.has(url.pathname)) return;
  event.respondWith(caches.open(CACHE).then(async cache => {
    const hit = await cache.match(url.pathname);
    if (hit) return hit;
    const response = await fetch(event.request);
    if (response.ok) await cache.put(url.pathname, response.clone());
    return response;
  }));
});
`);
console.log(`Asset manifest: ${assets.length} files, ${(assets.reduce((n,a)=>n+a.bytes,0)/1048576).toFixed(1)} MB (${version})`);
