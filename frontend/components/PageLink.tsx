'use client';

import NextLink from 'next/link';
import { useRouter } from 'next/navigation';
import { ComponentProps, useEffect, useState } from 'react';
import { withPageTimestamp } from '@/lib/page-url';

type Props = Omit<ComponentProps<typeof NextLink>, 'href'> & { href: string };

export default function PageLink({ href, onClick, replace, scroll, ...props }: Props) {
  const router = useRouter();
  const [target, setTarget] = useState(href);
  useEffect(() => { setTarget(withPageTimestamp(href)); }, [href]);

  return <NextLink {...props} href={target} replace={replace} scroll={scroll} prefetch={false} onClick={event => {
    onClick?.(event);
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey ||
        (props.target && props.target !== '_self') || props.download || !href.startsWith('/') || href.startsWith('//')) return;
    event.preventDefault();
    router[replace ? 'replace' : 'push'](withPageTimestamp(href), { scroll });
  }} />;
}
