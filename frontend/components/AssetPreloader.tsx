'use client';

import { ReactNode, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';

type Manifest = { version: string; assets: { url: string; bytes: number }[] };

export default function AssetPreloader({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [attempt, setAttempt] = useState(0);
  const [progress, setProgress] = useState({ done: 0, total: 0 });
  const [ready, setReady] = useState(false);
  const [error, setError] = useState(false);
  const [persistent, setPersistent] = useState(true);

  useEffect(() => {
    const abort = new AbortController();
    let cancelled = false;
    let fontRetryTimer: number | undefined;
    setError(false);
    setProgress({ done: 0, total: 0 });
    async function load() {
      const timedFetch = async (url: string, options: RequestInit = {}) => {
        const controller = new AbortController();
        const cancel = () => controller.abort();
        abort.signal.addEventListener('abort', cancel);
        const timer = window.setTimeout(cancel, 45000);
        try {
          const response = await fetch(url, { ...options, signal: controller.signal });
          if (!response.ok) throw new Error(`Asset failed: ${url}`);
          // Consume bodies before counting a resource as loaded.
          const blob = await response.blob();
          return new Response(blob, { status: response.status, headers: response.headers });
        } finally {
          clearTimeout(timer);
          abort.signal.removeEventListener('abort', cancel);
        }
      };
      try {
        const manifest: Manifest = await (await timedFetch('/asset-manifest.json', { cache: 'no-store' })).json();
        if (!manifest.assets?.length) throw new Error('Empty manifest');
        const cacheName = `home-assets-${manifest.version}`;
        let cache: Cache | undefined;
        try { cache = await caches.open(cacheName); } catch { setPersistent(false); }
        if (cancelled) return;
        setProgress({ done: 0, total: manifest.assets.length });
        let index = 0;
        let done = 0;
        let failed = false;
        const deferredFonts: string[] = [];
        await Promise.all(Array.from({ length: 4 }, async () => {
          while (index < manifest.assets.length && !cancelled) {
            const asset = manifest.assets[index++];
            const isFont = /\.(woff2?|ttf|otf|eot)(?:[?#]|$)/i.test(asset.url);
            const isImage = /\.(png|jpe?g|webp|gif|avif|svg|ico)(?:[?#]|$)/i.test(asset.url);
            let loaded = false;
            // Images get two automatic retries. Fonts must never block entry.
            for (let retry = 0; retry < (isImage ? 3 : 1) && !cancelled; retry++) {
              try {
                if (!(await cache?.match(asset.url))) {
                  const response = await timedFetch(asset.url, { cache: 'reload' });
                  try { await cache?.put(asset.url, response); }
                  catch { setPersistent(false); }
                }
                loaded = true;
                break;
              } catch { /* Retry failed image downloads before showing an error. */ }
            }
            if (loaded || isFont) {
              if (!loaded) deferredFonts.push(asset.url);
              done++;
              if (!cancelled) setProgress({ done, total: manifest.assets.length });
            } else { failed = true; }
          }
        }));
        if (cancelled) return;
        if (failed) throw new Error('Incomplete preload');
        if (cache && 'serviceWorker' in navigator) {
          try {
            const registration = await navigator.serviceWorker.register('/asset-worker.js', { updateViaCache: 'none' });
            const worker = registration.installing || registration.waiting || registration.active;
            if (worker && worker.state !== 'activated') {
              await new Promise<void>((resolve, reject) => {
                const timer = window.setTimeout(() => { worker.removeEventListener('statechange', changed); reject(new Error('Worker activation timeout')); }, 10000);
                function changed() {
                  if (worker?.state === 'activated' || worker?.state === 'redundant') {
                    clearTimeout(timer);
                    worker.removeEventListener('statechange', changed);
                    resolve();
                  }
                }
                worker.addEventListener('statechange', changed);
                changed();
              });
            }
          } catch { setPersistent(false); }
        }
        // Route data stays in Next's router cache, never in the public asset cache.
        router.prefetch('/problems');
        router.prefetch('/login');
        if (!cancelled) {
          setReady(true);
          // Non-blocking font retry after the visitor has entered the page.
          if (deferredFonts.length) fontRetryTimer = window.setTimeout(() => {
            deferredFonts.forEach(async url => {
              try { const response = await timedFetch(url); await cache?.put(url, response); }
              catch { /* System font fallback remains usable. */ }
            });
          }, 5000);
        }
      } catch {
        if (!cancelled) setError(true);
      }
    }
    void load();
    return () => { cancelled = true; abort.abort(); clearTimeout(fontRetryTimer); };
  }, [attempt, router]);

  if (ready) return <>{children}</>;
  const percent = progress.total ? Math.floor(progress.done / progress.total * 100) : 0;
  return (
    <main className="home-loading" aria-busy={!error}>
      <section className="home-loading-card">
        <span className="home-loading-mark" aria-hidden="true">✦</span>
        <p className="home-eyebrow">准备一点好心情</p>
        <h1>{error ? '加载暂时中断' : '资源加载中'}</h1>
        <p role="status">{error ? '请检查网络后重试，已缓存的资源无需重复下载。' : '首次加载可能需要一些时间，请稍候。'}</p>
        <div className="home-progress" role="progressbar" aria-label="静态资源加载进度" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent}>
          <span style={{ width: `${percent}%` }} />
        </div>
        <div className="home-progress-meta"><span>{progress.total ? `${progress.done} / ${progress.total} 项资源` : '正在获取资源清单'}</span><b>{percent}%</b></div>
        <p className="home-loading-note">{persistent ? '提前准备题目页与图片，下次打开更轻快。' : '浏览器未允许持久缓存，本次仍会完成资源加载。'}</p>
        {error && <button className="brand-primary-button px-6 py-3 mt-6" onClick={() => setAttempt(value => value + 1)}>重新加载</button>}
      </section>
    </main>
  );
}
