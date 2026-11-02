export function getSafeRedirectPath(value: string | null): string {
  if (!value) return '/problems';
  try {
    const base = new URL('https://local.invalid');
    const target = new URL(value, base);
    // Never allow external destinations or a login/register redirect loop.
    if (target.origin !== base.origin || /^\/(login|register)(\/|$)/.test(target.pathname)) return '/problems';
    return `${target.pathname}${target.search}${target.hash}`;
  } catch { return '/problems'; }
}

export function getLoginUrl(expired = false): string {
  const { pathname, search, hash } = window.location;
  if (pathname === '/login') return `${pathname}${search}${hash}`;
  const params = new URLSearchParams({ redirect: `${pathname}${search}${hash}` });
  if (expired) params.set('expired', 'true');
  return `/login?${params.toString()}`;
}
