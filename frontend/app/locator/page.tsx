'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { accountAPI } from '@/lib/api';
import { CHINA_PROVINCES, isInMainlandChina } from '@/lib/chinaRegions';

type MapProvider = 'baidu' | 'google';
type LocatorMode = MapProvider | 'manual';
type LatLng = { lat: number; lng: number };

interface LocatorSecretPayload {
  openButtonLabel: string;
  targetUrlBase: string;
  schemePrefix: string;
  notes: string[];
}

const PROVIDERS: Array<{ id: MapProvider; label: string }> = [
  { id: 'baidu', label: 'Baidu' },
  { id: 'google', label: 'Google' },
];

const MODES: Array<{ id: LocatorMode; label: string }> = [
  ...PROVIDERS,
  { id: 'manual', label: '手动输入' },
];

// Multi-layer encrypted payload (Layer 1: bit-rotation + affine shift;
// Layer 2: keyed Fisher-Yates transposition; Layer 3: RC4-drop512 + feedback chaining).
const ENCRYPTED_LOCATOR_PAYLOAD =
  'ZgqRaJebeJ/Cv7vkKdMEoQwqXFdnEiOSX2G6Xb5buqvzmbzrup4c907gr16hfsvenkKCe51yKUVCtlzr8Fo9Lv1vw8OEQBXzR1lgVYqV8YxHyYm+U0CqTZ1njddjWIV/FxwQbwBwAtqLXxghXCQtJlc83uilNqwuN8q5D+AhqWSgg01N97IN2EyLFrNlGXg5gl+PzEu9huspMNXcfGZvPi3/igv9VYKxHJDSU3mtrzSARmqjYaIaqAQE/Yv9J5TXy6d9Z4RjrPtr2pKLZ8rUrUZMwy++SczND7yUn3JOEx0meVO6lSS5YUnRGeh6+t+Ee2tQB+dfbEzd6ouzUMCjjEJvNP3tiAVZmJo61SaCPoio/0BDC8u96d3TfXOsJ2a9MiqZp1YDjLgKGl0KpSA49pPVRyy6l/NVNRs7LXLbO8zK5lYKOgEXOtFs1OqiRFBpEMf6BKSp3KN+gg==';

const LOCATOR_KEYS = {
  k1: process.env.NEXT_PUBLIC_LOCATOR_KEY_1 || '',
  k2: process.env.NEXT_PUBLIC_LOCATOR_KEY_2 || '',
  k3: process.env.NEXT_PUBLIC_LOCATOR_KEY_3 || '',
};

function toBytes(str: string): number[] {
  return Array.from(new TextEncoder().encode(str));
}

function fromBytes(bytes: number[]): string {
  return new TextDecoder().decode(new Uint8Array(bytes));
}

function base64ToBytes(b64: string): number[] {
  const bin = typeof window !== 'undefined' ? window.atob(b64) : Buffer.from(b64, 'base64').toString('binary');
  const out = new Array<number>(bin.length);
  for (let i = 0; i < bin.length; i++) {
    out[i] = bin.charCodeAt(i) & 0xff;
  }
  return out;
}

function fnvSeeds(key: string): [number, number] {
  const kb = toBytes(key);
  let h1 = 0x811c9dc5 >>> 0;
  let h2 = 0x811c9dc5 >>> 0;
  for (let i = 0; i < kb.length; i++) {
    h1 ^= kb[i];
    h1 = Math.imul(h1, 0x01000193) >>> 0;
    h2 ^= kb[kb.length - 1 - i] ^ (i * 0x9e3779b9);
    h2 = Math.imul(h2, 0x01000193) >>> 0;
  }
  return [h1 || 1, h2 || 2];
}

function xorshift32(seed: number): () => number {
  let x = seed >>> 0 || 1;
  return () => {
    x ^= (x << 13) >>> 0;
    x ^= x >>> 17;
    x ^= (x << 5) >>> 0;
    return x >>> 0;
  };
}

function decryptLayer1(bytes: number[], key: string): number[] {
  const kb = toBytes(key);
  return bytes.map((b, i) => {
    const k = kb[i % kb.length];
    const shift = (k % 7) + 1;
    const unadded = (b - k - ((i * 31) % 256) + 512) & 0xff;
    return ((unadded >>> shift) | (unadded << (8 - shift))) & 0xff;
  });
}

function buildPermutation(len: number, key: string): number[] {
  const [s1, s2] = fnvSeeds(key);
  const rand = xorshift32(s1 ^ s2);
  const idx = Array.from({ length: len }, (_, i) => i);
  for (let i = len - 1; i > 0; i--) {
    const j = rand() % (i + 1);
    const tmp = idx[i];
    idx[i] = idx[j];
    idx[j] = tmp;
  }
  return idx;
}

function decryptLayer2(bytes: number[], key: string): number[] {
  const perm = buildPermutation(bytes.length, key);
  const out = new Array<number>(bytes.length);
  for (let i = 0; i < perm.length; i++) {
    out[perm[i]] = bytes[i];
  }
  return out;
}

function rc4Stream(keyBytes: number[], drop: number): () => number {
  const S = Array.from({ length: 256 }, (_, i) => i);
  let j = 0;
  for (let i = 0; i < 256; i++) {
    j = (j + S[i] + keyBytes[i % keyBytes.length]) & 0xff;
    [S[i], S[j]] = [S[j], S[i]];
  }
  let i = 0;
  j = 0;
  const next = () => {
    i = (i + 1) & 0xff;
    j = (j + S[i]) & 0xff;
    [S[i], S[j]] = [S[j], S[i]];
    return S[(S[i] + S[j]) & 0xff];
  };
  for (let d = 0; d < drop; d++) next();
  return next;
}

function decryptLayer3(bytes: number[], key: string): number[] {
  const kb = toBytes(key);
  const next = rc4Stream(kb, 512);
  let prev = 0xa5;
  return bytes.map((c) => {
    const b = (c ^ next() ^ prev) & 0xff;
    prev = c;
    return b;
  });
}

function decryptLocatorPayload(): LocatorSecretPayload | null {
  const { k1, k2, k3 } = LOCATOR_KEYS;
  if (!k1 || !k2 || !k3) return null;
  try {
    const cipherBytes = base64ToBytes(ENCRYPTED_LOCATOR_PAYLOAD);
    const afterL3 = decryptLayer3(cipherBytes, k3);
    const afterL2 = decryptLayer2(afterL3, k2);
    const plainBytes = decryptLayer1(afterL2, k1);
    const parsed = JSON.parse(fromBytes(plainBytes)) as LocatorSecretPayload;
    if (
      typeof parsed?.openButtonLabel === 'string' &&
      typeof parsed?.targetUrlBase === 'string' &&
      typeof parsed?.schemePrefix === 'string' &&
      Array.isArray(parsed?.notes)
    ) {
      return parsed;
    }
    return null;
  } catch {
    return null;
  }
}

function parseManualPosition(latText: string, lngText: string): LatLng | null {
  if (!latText.trim() || !lngText.trim()) return null;
  const lat = Number(latText);
  const lng = Number(lngText);
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) return null;
  if (lat < -90 || lat > 90 || lng < -180 || lng > 180) return null;
  return { lat, lng };
}

// Random offset within ±0.0234 with 0.0001 granularity (integer steps in [-234, 234]).
function randomCoordOffset(): number {
  const steps = Math.floor(Math.random() * 469) - 234;
  return steps / 10000;
}

function randomizeCoord(baseLat: number, baseLng: number): { lat: string; lng: string } {
  const lat = Math.min(90, Math.max(-90, baseLat + randomCoordOffset()));
  const lng = Math.min(180, Math.max(-180, baseLng + randomCoordOffset()));
  return {
    lat: lat.toFixed(4),
    lng: lng.toFixed(4),
  };
}

const DEFAULT_CENTER: LatLng = { lat: 39.915, lng: 116.404 };
const DEFAULT_ZOOM = 12;

const MAP_KEYS: Record<MapProvider, string | undefined> = {
  baidu: process.env.NEXT_PUBLIC_BAIDU_MAP_AK,
  google: process.env.NEXT_PUBLIC_GOOGLE_MAPS_KEY,
};

const KEY_ENV_NAMES: Record<MapProvider, string> = {
  baidu: 'NEXT_PUBLIC_BAIDU_MAP_AK',
  google: 'NEXT_PUBLIC_GOOGLE_MAPS_KEY',
};

const sdkPromises: Partial<Record<MapProvider, Promise<void>>> = {};

// Both SDKs signal readiness through a global callback rather than script onload,
// because their bootstrap scripts load further scripts asynchronously.
function loadMapSdk(provider: MapProvider, key: string): Promise<void> {
  const existing = sdkPromises[provider];
  if (existing) return existing;

  const promise = new Promise<void>((resolve, reject) => {
    const callbackName = `__locator_${provider}_ready`;
    (window as any)[callbackName] = () => resolve();
    const script = document.createElement('script');
    script.async = true;
    script.src = provider === 'baidu'
      ? `https://api.map.baidu.com/api?v=3.0&ak=${encodeURIComponent(key)}&callback=${callbackName}`
      : `https://maps.googleapis.com/maps/api/js?key=${encodeURIComponent(key)}&callback=${callbackName}`;
    script.onerror = () => {
      delete sdkPromises[provider];
      script.remove();
      reject(new Error('地图脚本加载失败'));
    };
    document.head.appendChild(script);
  });
  sdkPromises[provider] = promise;
  return promise;
}

function buildTargetAppUrl(secret: LocatorSecretPayload, mode: LocatorMode, { lat, lng }: LatLng): string {
  const geotype = mode === 'google' && !isInMainlandChina(lat, lng) ? 'WGS84' : 'GCJ02';
  const targetUrl = `${secret.targetUrlBase}?lng=${lng.toFixed(6)}&lat=${lat.toFixed(6)}&geotype=${geotype}`;
  return `${secret.schemePrefix}${encodeURIComponent(targetUrl)}`;
}

function MapPicker({ provider, onPick }: { provider: MapProvider; onPick: (position: LatLng) => void }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const onPickRef = useRef(onPick);
  onPickRef.current = onPick;
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState('');
  const key = MAP_KEYS[provider];

  useEffect(() => {
    if (!key) {
      setStatus('error');
      setError(`未配置地图密钥，请设置环境变量 ${KEY_ENV_NAMES[provider]}`);
      return;
    }

    let cancelled = false;
    let cleanup = () => {};
    setStatus('loading');
    setError('');

    loadMapSdk(provider, key)
      .then(() => {
        const container = containerRef.current;
        if (cancelled || !container) return;

        if (provider === 'baidu') {
          const BMap = (window as any).BMap;
          const map = new BMap.Map(container);
          map.centerAndZoom(new BMap.Point(DEFAULT_CENTER.lng, DEFAULT_CENTER.lat), DEFAULT_ZOOM);
          map.enableScrollWheelZoom(true);
          let marker: any = null;
          const handleClick = (event: any) => {
            if (marker) map.removeOverlay(marker);
            marker = new BMap.Marker(event.point);
            map.addOverlay(marker);
            onPickRef.current({ lat: event.point.lat, lng: event.point.lng });
          };
          map.addEventListener('click', handleClick);
          cleanup = () => map.removeEventListener('click', handleClick);
        } else {
          const google = (window as any).google;
          const map = new google.maps.Map(container, { center: DEFAULT_CENTER, zoom: DEFAULT_ZOOM, streetViewControl: false });
          const marker = new google.maps.Marker({ map });
          const listener = map.addListener('click', (event: any) => {
            const position = { lat: event.latLng.lat(), lng: event.latLng.lng() };
            marker.setPosition(position);
            onPickRef.current(position);
          });
          cleanup = () => listener.remove();
        }
        setStatus('ready');
      })
      .catch((loadError: Error) => {
        if (cancelled) return;
        setStatus('error');
        setError(loadError.message);
      });

    return () => {
      cancelled = true;
      cleanup();
    };
  }, [provider, key]);

  return (
    <div className="relative h-[420px] w-full overflow-hidden rounded-2xl border border-gray-200 dark:border-gray-700 bg-gray-100 dark:bg-gray-800">
      <div ref={containerRef} className="h-full w-full" />
      {status !== 'ready' && (
        <div className="absolute inset-0 flex items-center justify-center px-6 text-center text-sm text-gray-500 dark:text-gray-400">
          {status === 'loading' ? '地图加载中…' : error}
        </div>
      )}
    </div>
  );
}

export default function LocatorPage() {
  const router = useRouter();
  const { isAuthenticated, loading, user } = useAuth();
  const { t } = useLanguage();
  const [verified, setVerified] = useState(false);
  const [mode, setMode] = useState<LocatorMode>('manual');
  const [position, setPosition] = useState<LatLng | null>(null);
  const [selectedProvince, setSelectedProvince] = useState('');
  const [selectedCity, setSelectedCity] = useState('');
  const [manualLat, setManualLat] = useState('');
  const [manualLng, setManualLng] = useState('');

  const secretPayload = useMemo(() => decryptLocatorPayload(), []);

  useEffect(() => {
    if (!loading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, loading, router]);

  // The stored token is only trusted after the backend accepts it;
  // apiRequest redirects to /login on 401.
  useEffect(() => {
    if (loading || !isAuthenticated || !user) return;
    if (process.env.NEXT_PUBLIC_UI_PREVIEW === '1') {
      setVerified(true);
      return;
    }
    let cancelled = false;
    accountAPI.getAccountPoints(user.id)
      .then(() => { if (!cancelled) setVerified(true); })
      .catch((error) => {
        if (!cancelled) console.error('Failed to verify login status:', error);
      });
    return () => { cancelled = true; };
  }, [isAuthenticated, loading, user]);

  if (loading || (isAuthenticated && !verified)) {
    return (
      <div className="min-h-screen flex items-center justify-center dark:bg-gray-950">
        <div className="text-gray-600 dark:text-gray-300">{t('loading')}</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  const selectMode = (next: LocatorMode) => {
    if (next === mode) return;
    setPosition(null);
    setMode(next);
  };

  const activeProvinceObj = CHINA_PROVINCES.find((item) => item.name === selectedProvince);
  const availableCities = activeProvinceObj?.cities ?? [];

  const handleProvinceChange = (provinceName: string) => {
    setSelectedProvince(provinceName);
    const province = CHINA_PROVINCES.find((item) => item.name === provinceName);
    if (!province) {
      setSelectedCity('');
      return;
    }
    const firstCity = province.cities[0];
    if (firstCity) {
      setSelectedCity(firstCity.name);
      const randomized = randomizeCoord(firstCity.lat, firstCity.lng);
      setManualLat(randomized.lat);
      setManualLng(randomized.lng);
    } else {
      setSelectedCity('');
      const randomized = randomizeCoord(province.lat, province.lng);
      setManualLat(randomized.lat);
      setManualLng(randomized.lng);
    }
  };

  const handleCityChange = (cityName: string) => {
    setSelectedCity(cityName);
    const city = availableCities.find((item) => item.name === cityName);
    if (!city) return;
    const randomized = randomizeCoord(city.lat, city.lng);
    setManualLat(randomized.lat);
    setManualLng(randomized.lng);
  };

  const manualPosition = parseManualPosition(manualLat, manualLng);
  const manualInvalid = Boolean(manualLat.trim() && manualLng.trim()) && !manualPosition;
  const inputClassName = 'w-full px-4 py-2 rounded-xl border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 font-mono outline-none focus:ring-2 focus:ring-yellow-400';
  const selectClassName = 'w-full px-4 py-2 rounded-xl border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 outline-none focus:ring-2 focus:ring-yellow-400';
  const openLinkClassName = 'brand-primary-button px-5 py-2 text-sm';

  return (
    <div className="brand-page min-h-screen">
      <PageTitle titleKey="pageTitle.locator" />
      <Navbar title="定位" showBackButton backHref="/problems" />

      <main className="max-w-4xl mx-auto px-4 py-6">
        <section className="brand-card p-4 sm:p-6 space-y-5">
          <div className="flex flex-wrap gap-3">
            {MODES.map(({ id, label }) => (
              <button
                key={id}
                type="button"
                onClick={() => selectMode(id)}
                className={`px-5 py-2 rounded-xl font-semibold transition ${
                  mode === id
                    ? 'brand-primary-button'
                    : 'bg-white/70 dark:bg-gray-800 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-700 hover:bg-white dark:hover:bg-gray-700'
                }`}
                aria-pressed={mode === id}
              >
                {label}
              </button>
            ))}
          </div>

          {mode === 'manual' ? (
            <div className="space-y-3">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <label className="block text-sm text-gray-700 dark:text-gray-300">
                  <span className="block mb-1">省份</span>
                  <select
                    value={selectedProvince}
                    onChange={(event) => handleProvinceChange(event.target.value)}
                    className={selectClassName}
                  >
                    <option value="">请选择省份</option>
                    {CHINA_PROVINCES.map((province) => (
                      <option key={province.name} value={province.name}>
                        {province.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="block text-sm text-gray-700 dark:text-gray-300">
                  <span className="block mb-1">城市 / 地区</span>
                  <select
                    value={selectedCity}
                    onChange={(event) => handleCityChange(event.target.value)}
                    disabled={!selectedProvince}
                    className={`${selectClassName} disabled:opacity-50 disabled:cursor-not-allowed`}
                  >
                    <option value="">{selectedProvince ? '请选择城市' : '请先选择省份'}</option>
                    {availableCities.map((city) => (
                      <option key={city.name} value={city.name}>
                        {city.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <label className="block text-sm text-gray-700 dark:text-gray-300">
                  <span className="block mb-1">纬度（-90 ~ 90）</span>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={manualLat}
                    onChange={(event) => setManualLat(event.target.value)}
                    placeholder="例如 39.915"
                    className={inputClassName}
                  />
                </label>
                <label className="block text-sm text-gray-700 dark:text-gray-300">
                  <span className="block mb-1">经度（-180 ~ 180）</span>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={manualLng}
                    onChange={(event) => setManualLng(event.target.value)}
                    placeholder="例如 116.404"
                    className={inputClassName}
                  />
                </label>
              </div>
              <div className="flex flex-wrap items-center gap-3">
                {manualPosition ? (
                  secretPayload ? (
                    <a
                      href={buildTargetAppUrl(secretPayload, 'manual', manualPosition)}
                      className={openLinkClassName}
                    >
                      {secretPayload.openButtonLabel}
                    </a>
                  ) : (
                    <span className="text-sm text-red-600 dark:text-red-400">未配置解密密钥，无法生成打开链接</span>
                  )
                ) : (
                  <span className={`text-sm ${manualInvalid ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'}`}>
                    {manualInvalid ? '经纬度格式不正确或超出范围' : '请输入纬度和经度，或从上方选择省市'}
                  </span>
                )}
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <MapPicker key={mode} provider={mode} onPick={setPosition} />
              <div className="flex flex-wrap items-center gap-3">
                <div className="text-sm text-gray-700 dark:text-gray-300">
                  {position ? (
                    <>
                      <span className="mr-4">纬度 <b className="font-mono">{position.lat.toFixed(6)}</b></span>
                      <span>经度 <b className="font-mono">{position.lng.toFixed(6)}</b></span>
                    </>
                  ) : (
                    <span className="text-gray-500 dark:text-gray-400">点击地图选择位置以获取经纬度</span>
                  )}
                </div>
                {position && (
                  secretPayload ? (
                    <a
                      href={buildTargetAppUrl(secretPayload, mode, position)}
                      className={openLinkClassName}
                    >
                      {secretPayload.openButtonLabel}
                    </a>
                  ) : (
                    <span className="text-sm text-red-600 dark:text-red-400">未配置解密密钥，无法生成打开链接</span>
                  )
                )}
              </div>
            </div>
          )}
        </section>

        <section className="brand-card p-4 sm:p-6 mt-6">
          <h2 className="text-lg font-bold mb-2">说明</h2>
          <div className="text-sm text-gray-600 dark:text-gray-400 space-y-2">
            {secretPayload ? (
              secretPayload.notes.map((line, index) => <p key={index}>{line}</p>)
            ) : (
              <p>未配置解密密钥，说明内容已加密隐藏。</p>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}
