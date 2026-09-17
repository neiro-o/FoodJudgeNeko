'use client';

import { useEffect } from 'react';

const LARGE_ASSETS = [
  '/brand/kangaroo-reader.png?v=2',
  '/brand/book-stack.png',
  '/brand/kangaroo-milk-tea.png',
];

export default function AssetPreloader() {
  useEffect(() => {
    const preload = () => {
      LARGE_ASSETS.forEach((src) => {
        const image = new Image();
        image.decoding = 'async';
        image.src = src;
      });
    };

    const browserWindow = window as Window & {
      requestIdleCallback?: (callback: () => void, options?: { timeout: number }) => number;
      cancelIdleCallback?: (id: number) => void;
    };

    if (browserWindow.requestIdleCallback) {
      const id = browserWindow.requestIdleCallback(preload, { timeout: 1800 });
      return () => browserWindow.cancelIdleCallback?.(id);
    }

    const id = window.setTimeout(preload, 350);
    return () => window.clearTimeout(id);
  }, []);

  return null;
}
