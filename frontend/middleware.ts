import { NextRequest, NextResponse } from 'next/server';

export function middleware(request: NextRequest) {
  const url = request.nextUrl.clone();
  const needsTimestamp = !/^\d{13,}$/.test(url.searchParams.get('_t') || '');
  if (needsTimestamp) {
    // Standalone Next uses its internal listener as nextUrl's origin. The
    // gateway preserves Host; public pages use HTTPS even behind an HTTP hop.
    const host = request.headers.get('host') || url.host;
    url.host = host;
    url.port = new URL(`http://${host}`).port;
    if (!/^(localhost|127\.0\.0\.1|\[::1\])(?::\d+)?$/.test(host) && process.env.NODE_ENV === 'production') {
      url.protocol = 'https:';
    }
    url.searchParams.set('_t', String(Date.now()));
  }
  const response = needsTimestamp ? NextResponse.redirect(url, 307) : NextResponse.next();
  response.headers.set('Cache-Control', 'private, no-store, max-age=0, must-revalidate');
  response.headers.set('CDN-Cache-Control', 'no-store');
  return response;
}

export const config = {
  matcher: ['/((?!api(?:/|$)|_next(?:/|$)|.*\\.).*)'],
};
