import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import test from 'node:test';

const require = createRequire(import.meta.url);
const ts = require('typescript');
const storage = () => {
  const values = new Map();
  return { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: key => values.delete(key) };
};
const user = { id: 'test-user', username: 'test', email: '', points: 0, is_admin: false };
const credentials = { username: 'test', password: 'test-only-password' };
const jwt = exp => `header.${Buffer.from(JSON.stringify({ exp })).toString('base64url')}.signature`;
const loginResponse = { token: jwt(Date.now() / 1000 + 86400), user };

function environment() {
  let now = Date.now();
  const localStorage = storage(), sessionStorage = storage(), redirects = [];
  const context = vm.createContext({
    localStorage, sessionStorage, URL, URLSearchParams, atob, Event,
    Date: class extends Date { static now() { return now; } },
    process: { env: {} },
    window: {
      dispatchEvent() {},
      location: { pathname: '/user_stats/123', search: '?page=2&name=%E4%B8%AD', hash: '#comments', replace: url => redirects.push(url) },
    },
    fetch: (...args) => context.mockFetch(...args),
  });
  const cache = new Map();
  function load(name) {
    if (cache.has(name)) return cache.get(name);
    const source = readFileSync(fileURLToPath(new URL(`../lib/${name}.ts`, import.meta.url)), 'utf8');
    const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
    const module = { exports: {} };
    cache.set(name, module.exports);
    vm.runInContext(`(function(require, module, exports) {${output}\n})`, context)(path => load(path.replace('./', '')), module, module.exports);
    return module.exports;
  }
  return { auth: load('auth-storage'), api: load('api'), redirect: load('auth-redirect'), localStorage, sessionStorage, redirects, context, advance: ms => { now += ms; } };
}

const response = (status, data) => ({ status, ok: status >= 200 && status < 300, json: async () => data });

test('remember password saves immediately, edits and uncheck remove it independently', () => {
  const { auth } = environment();
  auth.rememberLogin(credentials);
  assert.equal(auth.getRememberedLogin().password, credentials.password);
  auth.rememberLogin({ ...credentials, password: 'edited' });
  assert.equal(auth.getRememberedLogin().password, 'edited');
  auth.rememberLogin(null);
  assert.equal(auth.getRememberedLogin(), null);
});

test('ordinary login lives in session storage; remembering does not enable auto login', () => {
  const { auth, localStorage, sessionStorage } = environment();
  auth.startLogin(loginResponse, credentials, { rememberPassword: true, autoLogin: false });
  assert.equal(localStorage.getItem('token'), null);
  assert.equal(sessionStorage.getItem('token'), loginResponse.token);
  assert.equal(auth.getAutoLogin(), null);
  assert.equal(auth.getAuthToken(), loginResponse.token);
});

test('120h has a fixed boundary and renewal does not extend it', () => {
  const { auth, advance } = environment();
  auth.startLogin(loginResponse, credentials, { rememberPassword: false, autoLogin: true });
  const deadline = auth.getAutoLogin().expiresAt;
  assert.equal(auth.getRememberedLogin(), null);
  advance(auth.AUTO_LOGIN_DURATION - 1);
  auth.saveAuth(loginResponse);
  assert.equal(auth.getAutoLogin().expiresAt, deadline);
  assert.equal(auth.getAuthToken(), loginResponse.token);
  advance(1);
  assert.equal(auth.getAutoLogin(), null);
  assert.equal(auth.getAuthToken(), null);
});

test('logout removes automatic credentials while keeping separately remembered password', () => {
  const { auth } = environment();
  auth.startLogin(loginResponse, credentials, { rememberPassword: true, autoLogin: true });
  auth.clearAuth();
  assert.equal(auth.getAutoLogin(), null);
  assert.equal(auth.getAuthToken(), null);
  assert.equal(auth.getRememberedLogin().password, credentials.password);
});

test('corrupt storage and old unlimited local sessions do not restore authentication', () => {
  const { auth, localStorage } = environment();
  localStorage.setItem('token', loginResponse.token);
  localStorage.setItem('user', JSON.stringify(user));
  localStorage.setItem('autoLoginV1', '{invalid');
  assert.equal(auth.getStoredAuth(), null);
});

test('safe return paths retain query/hash and reject external or auth page loops', () => {
  const { redirect } = environment();
  for (const target of [null, 'https://evil.invalid/x', '//evil.invalid/x', '/login?redirect=/user', '/register', 'javascript:alert(1)']) {
    assert.equal(redirect.getSafeRedirectPath(target), '/problems');
  }
  assert.equal(redirect.getSafeRedirectPath('/user_stats/123?page=2#comments'), '/user_stats/123?page=2#comments');
  const url = new URL(redirect.getLoginUrl(true), 'https://app.invalid');
  assert.equal(url.searchParams.get('redirect'), '/user_stats/123?page=2&name=%E4%B8%AD#comments');
  assert.equal(url.searchParams.get('expired'), 'true');
});

test('401 without auto login clears session and redirects to the complete original URL', async () => {
  const e = environment();
  e.auth.startLogin(loginResponse, credentials, { rememberPassword: false, autoLogin: false });
  e.context.mockFetch = async () => response(401, {});
  await assert.rejects(e.api.searchAPI.quota(), /Unauthorized/);
  assert.equal(e.auth.getAuthToken(), null);
  assert.equal(new URL(e.redirects[0], 'https://app.invalid').searchParams.get('redirect'), '/user_stats/123?page=2&name=%E4%B8%AD#comments');
});

test('concurrent 401 requests share one renewal and retry with its new token', async () => {
  const e = environment();
  e.auth.startLogin(loginResponse, credentials, { rememberPassword: false, autoLogin: true });
  const deadline = e.auth.getAutoLogin().expiresAt;
  let logins = 0, calls = 0;
  const renewed = jwt(Date.now() / 1000 + 172800);
  e.context.mockFetch = async (url, options) => {
    if (url === '/api/login') {
      logins++;
      assert.deepEqual(JSON.parse(options.body), credentials);
      await new Promise(resolve => setImmediate(resolve));
      return response(200, { code: 0, data: { token: renewed, user } });
    }
    calls++;
    return options.headers.Authorization === `Bearer ${renewed}`
      ? response(200, { code: 0, data: { unlimited: true } }) : response(401, {});
  };
  await Promise.all([e.api.searchAPI.quota(), e.api.searchAPI.quota()]);
  assert.equal(logins, 1);
  assert.equal(calls, 4);
  assert.equal(e.redirects.length, 0);
  assert.equal(e.auth.getAutoLogin().expiresAt, deadline);
});

test('24h token expiry silently logs in again inside the 120h window', async () => {
  const e = environment();
  e.auth.startLogin({ token: jwt(Date.now() / 1000 - 1), user }, credentials, { rememberPassword: false, autoLogin: true });
  let logins = 0;
  e.context.mockFetch = async url => {
    if (url === '/api/login') { logins++; return response(200, { code: 0, data: loginResponse }); }
    return response(200, { code: 0, data: { unlimited: true } });
  };
  await e.api.searchAPI.quota();
  assert.equal(logins, 1);
  assert.equal(e.redirects.length, 0);
});

test('failed renewal is bounded and a rejected manual login stays on the form', async () => {
  const e = environment();
  e.auth.startLogin(loginResponse, credentials, { rememberPassword: true, autoLogin: true });
  let calls = 0;
  e.context.mockFetch = async () => { calls++; return response(401, { code: 401, message: 'Wrong password' }); };
  await assert.rejects(e.api.searchAPI.quota(), /Unauthorized/);
  assert.equal(calls, 2);
  assert.equal(e.auth.getAutoLogin(), null);
  assert.equal(e.redirects.length, 1);
  await assert.rejects(e.api.authAPI.login(credentials), /Wrong password/);
  assert.equal(e.redirects.length, 1);
});

test('logout during an in-flight renewal cannot restore the session', async () => {
  const e = environment();
  e.auth.startLogin(loginResponse, credentials, { rememberPassword: false, autoLogin: true });
  let resolve;
  e.context.mockFetch = () => new Promise(done => { resolve = done; });
  const pending = e.api.restoreAutomaticLogin();
  e.auth.clearAuth();
  resolve(response(200, { code: 0, data: loginResponse }));
  assert.equal(await pending, null);
  assert.equal(e.auth.getAuthToken(), null);
});

test('a renewal waiting on another tab reuses that tab\'s new token', async () => {
  const e = environment();
  e.auth.startLogin(loginResponse, credentials, { rememberPassword: false, autoLogin: true });
  let run;
  e.context.navigator = { locks: { request: (_name, callback) => new Promise(resolve => { run = () => resolve(callback()); }) } };
  e.context.mockFetch = () => assert.fail('Another tab has already renewed the session');
  const pending = e.api.restoreAutomaticLogin();
  const renewed = jwt(Date.now() / 1000 + 172800);
  e.auth.saveAuth({ token: renewed, user });
  run();
  assert.equal((await pending).token, renewed);
});
