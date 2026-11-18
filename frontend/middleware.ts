import { NextRequest, NextResponse } from 'next/server';

export function middleware(request: NextRequest) {
  const url = request.nextUrl.clone();
  const needsTimestamp = !/^\d{13,}$/.test(url.searchParams.get('_t') || '');
  if (needsTimestamp) url.searchParams.set('_t', String(Date.now()));
  const response = needsTimestamp ? NextResponse.redirect(url, 307) : NextResponse.next();
  response.headers.set('Cache-Control', 'private, no-store, max-age=0, must-revalidate');
  response.headers.set('CDN-Cache-Control', 'no-store');
  return response;
}

export const config = {
  matcher: ['/((?!api(?:/|$)|_next(?:/|$)|.*\\.).*)'],
};
