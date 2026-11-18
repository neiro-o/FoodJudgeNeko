// Apply only to local page routes; APIs and versioned static assets keep their URLs.
export function withPageTimestamp(href: string, now = Date.now()): string {
  if (!href.startsWith('/') || href.startsWith('//')) return href;
  const url = new URL(href, 'https://local.invalid');
  if (/^\/(api|_next)(\/|$)/.test(url.pathname) || url.pathname.includes('.')) return href;
  url.searchParams.set('_t', String(now));
  return `${url.pathname}${url.search}${url.hash}`;
}
