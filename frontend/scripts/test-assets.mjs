import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
const manifest = JSON.parse(await readFile('public/asset-manifest.json', 'utf8'));
assert(manifest.assets.some(a => /chunks\/app\/problems\//.test(a.url)), 'Problem routes must be preloaded');
assert.equal(manifest.assets.filter(a => a.url.startsWith('/landing/')).length, 6);
assert(!manifest.assets.some(a => a.url.includes('le0ou') || a.url.startsWith('/api/')));
const handlers = {};
const hit = new Response('cached');
let fetched = false;
vm.runInNewContext(await readFile('public/asset-worker.js', 'utf8'), {
  URL,
  self: { location: { origin: 'https://test.invalid' }, addEventListener: (name, fn) => handlers[name] = fn },
  caches: { open: async () => ({ match: async () => hit }) },
  fetch: async () => { fetched = true; throw Error('Network should not be used for a cached asset'); },
});
for (const url of ['https://test.invalid/api/user', 'https://test.invalid/login', 'https://test.invalid/problems?_rsc=test', 'https://external.invalid/landing/classic.png']) {
  handlers.fetch({ request: { url, method: 'GET' }, respondWith: () => assert.fail(`Unexpected cache interception: ${url}`) });
}
let result;
handlers.fetch({ request: { url: 'https://test.invalid/brand/kangaroo-reader.png?v=2', method: 'GET' }, respondWith: promise => result = promise });
assert.equal(await result, hit);
assert.equal(fetched, false);
console.log('PASS: all six cutouts and problem chunks in manifest; private requests excluded; versioned image cache hit without network.');
