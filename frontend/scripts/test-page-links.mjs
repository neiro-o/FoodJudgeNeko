import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import vm from 'node:vm';
import test from 'node:test';

const require = createRequire(import.meta.url);
const ts = require('typescript');
function load(path, imports = {}) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8');
  const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
  const module = { exports: {} };
  vm.runInNewContext(`(function(require, module, exports) {${output}\n})`, { URL, Date, atob })(name => imports[name], module, module.exports);
  return module.exports;
}
const { withPageTimestamp } = load('../lib/page-url.ts');
const { buildProblemAppLink } = load('../lib/problem-link.ts');

test('page navigations get a fresh timestamp while preserving query and fragment', () => {
  for (const path of ['/problems', '/locator', '/login', '/problems/abc', '/user_stats/123']) {
    const first = withPageTimestamp(`${path}?q=%E4%B8%AD&_t=old#comments`, 1794981600000);
    const second = withPageTimestamp(first, 1794981600100);
    const url = new URL(second, 'https://test.invalid');
    assert.equal(url.pathname, path);
    assert.equal(url.searchParams.get('q'), '中');
    assert.deepEqual(url.searchParams.getAll('_t'), ['1794981600100']);
    assert.equal(url.hash, '#comments');
  }
});

test('API, immutable assets and external/deep links keep their original URLs', () => {
  for (const href of ['/api/problem/link/123', '/_next/static/chunk.js', '/icon.png', 'https://example.invalid/path', '//example.invalid/path', '#comments', 'other-app://open']) {
    assert.equal(withPageTimestamp(href), href);
  }
});

test('app link encodes the complete nested URL exactly once', () => {
  const source = new URL('/xiaomei/static/inbox-task-detail', `https://${['zqt', 'meituan', 'com'].join('.')}`);
  source.searchParams.set('targetReplyId', 'reply+?#&');
  source.searchParams.set('voteTaskNo', 'TASK 中文');
  const link = buildProblemAppLink(source.href);
  const parsed = new URL(link);
  assert.equal(parsed.protocol, ['i', 'meituan', ':'].join(''));
  assert.equal(parsed.hostname, ['www', 'meituan', 'com'].join('.'));
  assert.equal(parsed.pathname, '/web');
  assert.equal(parsed.searchParams.get('url'), source.href);
  assert.ok(link.endsWith(encodeURIComponent(source.href)));
});

class MockURL extends URL { clone() { return new MockURL(this.href); } }
const { middleware } = load('../middleware.ts', { 'next/server': {
  NextResponse: {
    redirect: (url, status) => ({ url, status, headers: new Map() }),
    next: () => ({ status: 200, headers: new Map() }),
  },
} });

test('direct page visits redirect once with no-store and retain search parameters', () => {
  const response = middleware({ nextUrl: new MockURL('https://test.invalid/login?redirect=%2Fproblems%3Fq%3Dabc') });
  assert.equal(response.status, 307);
  assert.match(response.url.searchParams.get('_t'), /^\d{13,}$/);
  assert.equal(response.url.searchParams.get('redirect'), '/problems?q=abc');
  assert.match(response.headers.get('Cache-Control'), /no-store/);
  assert.equal(response.headers.get('CDN-Cache-Control'), 'no-store');
  const next = middleware({ nextUrl: response.url });
  assert.equal(next.status, 200);
  assert.match(next.headers.get('Cache-Control'), /no-store/);
});
