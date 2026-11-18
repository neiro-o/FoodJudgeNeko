'use client';

import { useMemo } from 'react';
import { useRouter as useNextRouter } from 'next/navigation';
import { withPageTimestamp } from './page-url';

export * from 'next/navigation';

export function useRouter(): ReturnType<typeof useNextRouter> {
  const router = useNextRouter();
  return useMemo(() => ({
    ...router,
    push: (href, options) => router.push(withPageTimestamp(href), options),
    replace: (href, options) => router.replace(withPageTimestamp(href), options),
    prefetch: (href, options) => router.prefetch(withPageTimestamp(href), options),
  }), [router]);
}
