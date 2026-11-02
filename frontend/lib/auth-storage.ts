import type { LoginResponse } from './api';

export const AUTO_LOGIN_DURATION = 120 * 60 * 60 * 1000;
export const AUTH_CHANGED_EVENT = 'mtv2-auth-changed';
const AUTO_LOGIN_KEY = 'autoLoginV1';
const REMEMBER_PASSWORD_KEY = 'rememberedLoginV1';

export interface Credentials { username: string; password: string }
export interface LoginOptions { rememberPassword: boolean; autoLogin: boolean }
interface AutoLogin extends Credentials { expiresAt: number }

function readJSON<T>(key: string): T | null {
  if (typeof window === 'undefined') return null;
  try { return JSON.parse(localStorage.getItem(key) || 'null'); } catch { return null; }
}

function isCredentials(value: unknown): value is Credentials {
  const item = value as Credentials | null;
  return !!item && typeof item.username === 'string' && typeof item.password === 'string';
}

export function getRememberedLogin(): Credentials | null {
  const saved = readJSON<Credentials>(REMEMBER_PASSWORD_KEY);
  return isCredentials(saved) ? saved : null;
}

// The checkbox takes effect immediately, including edits before form submission.
export function rememberLogin(credentials: Credentials | null) {
  if (credentials) localStorage.setItem(REMEMBER_PASSWORD_KEY, JSON.stringify(credentials));
  else localStorage.removeItem(REMEMBER_PASSWORD_KEY);
}

export function clearAuth() {
  for (const storage of [localStorage, sessionStorage]) {
    storage.removeItem('token');
    storage.removeItem('user');
  }
  localStorage.removeItem(AUTO_LOGIN_KEY);
}

export function getAutoLogin(): AutoLogin | null {
  const saved = readJSON<AutoLogin>(AUTO_LOGIN_KEY);
  if (isCredentials(saved) && typeof saved.expiresAt === 'number' && saved.expiresAt > Date.now()) {
    return saved;
  }
  // Old unlimited local sessions and expired automatic sessions require a login.
  if (typeof window !== 'undefined') {
    localStorage.removeItem(AUTO_LOGIN_KEY);
    localStorage.removeItem('token');
    localStorage.removeItem('user');
  }
  return null;
}

export function getStoredAuth(): LoginResponse | null {
  if (typeof window === 'undefined') return null;
  try {
    const storage = getAutoLogin() ? localStorage : sessionStorage;
    const token = storage.getItem('token');
    const user = JSON.parse(storage.getItem('user') || 'null');
    return token && user && typeof user.id === 'string' ? { token, user } : null;
  } catch { return null; }
}

export function getAuthToken(): string | null {
  return getStoredAuth()?.token || null;
}

export function tokenIsExpired(token: string): boolean {
  try {
    const payload = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(payload)).exp * 1000 <= Date.now() + 30_000;
  } catch { return true; }
}

export function saveAuth(response: LoginResponse) {
  const storage = getAutoLogin() ? localStorage : sessionStorage;
  storage.setItem('token', response.token);
  storage.setItem('user', JSON.stringify(response.user));
  window.dispatchEvent(new Event(AUTH_CHANGED_EVENT));
}

export function startLogin(response: LoginResponse, credentials: Credentials, options: LoginOptions) {
  clearAuth();
  rememberLogin(options.rememberPassword ? credentials : null);
  if (options.autoLogin) {
    localStorage.setItem(AUTO_LOGIN_KEY, JSON.stringify({ ...credentials, expiresAt: Date.now() + AUTO_LOGIN_DURATION }));
  }
  saveAuth(response);
}
