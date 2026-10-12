'use client';

import { PreferenceImg } from '@/components/PreferenceImage';

import { useEffect, useState, useRef } from 'react';
import { createPortal } from 'react-dom';
import { useDisplayMode } from '@/contexts/DisplayModeContext';
import { mediaAPI } from '@/lib/api';

interface ImageModalProps {
  imageUrl: string;
  isOpen: boolean;
  onClose: () => void;
}

export default function ImageModal({ imageUrl, isOpen: requestedOpen, onClose }: ImageModalProps) {
  const { ready, isEnergySaving } = useDisplayMode();
  const isOpen = requestedOpen && ready && !isEnergySaving;
  const [isLoading, setIsLoading] = useState(true);
  const [proxiedUrl, setProxiedUrl] = useState<string>('');
  const [isVideo, setIsVideo] = useState(false);
  const [imageSize, setImageSize] = useState({ width: 0, height: 0 });
  const [viewportSize, setViewportSize] = useState({ width: 0, height: 0 });
  const [viewMode, setViewMode] = useState<'screen' | 'width' | 'custom'>('screen');
  const [customScale, setCustomScale] = useState(1);
  const [mediaError, setMediaError] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState('');
  const viewportRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);

  const screenScale = imageSize.width && imageSize.height && viewportSize.width && viewportSize.height
    ? Math.min(viewportSize.width / imageSize.width, viewportSize.height / imageSize.height)
    : 1;
  const widthScale = imageSize.width && viewportSize.width ? viewportSize.width / imageSize.width : 1;
  const scale = viewMode === 'screen' ? screenScale : viewMode === 'width' ? widthScale : customScale;
  const imageReady = !isLoading && !mediaError && imageSize.width > 0 && viewportSize.width > 0;
  const toolbarButtonClass = 'shrink-0 rounded-lg px-3 py-2 text-sm text-white transition hover:bg-white/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white disabled:cursor-not-allowed disabled:opacity-40';

  const zoom = (factor: number) => {
    setCustomScale(Math.min(16, Math.max(0.01, scale * factor)));
    setViewMode('custom');
  };

  const downloadImage = async () => {
    if (!proxiedUrl || isDownloading) return;
    setIsDownloading(true);
    setDownloadError('');
    try {
      const response = await fetch(proxiedUrl);
      if (!response.ok) throw new Error('下载失败');
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      const extensions: Record<string, string> = {
        'image/jpeg': 'jpg', 'image/png': 'png', 'image/gif': 'gif',
        'image/webp': 'webp', 'image/svg+xml': 'svg', 'image/avif': 'avif',
        'image/bmp': 'bmp', 'image/tiff': 'tiff',
      };
      const originalName = new URL(imageUrl, window.location.href).pathname.split('/').pop() || '';
      link.href = url;
      link.download = /\.(jpe?g|png|gif|webp|svg|avif|bmp|tiff?)$/i.test(originalName)
        ? originalName : `image.${extensions[blob.type] || 'png'}`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      setDownloadError('图片下载失败，请重试');
    } finally {
      setIsDownloading(false);
    }
  };

  // Measure the space remaining above the toolbar, including after window resizing.
  useEffect(() => {
    if (!isOpen || !viewportRef.current) return;
    const viewport = viewportRef.current;
    const updateSize = () => setViewportSize({ width: viewport.clientWidth, height: viewport.clientHeight });
    updateSize();
    const observer = new ResizeObserver(updateSize);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, [isOpen]);

  useEffect(() => {
    if (isOpen) viewportRef.current?.scrollTo({ top: 0, left: 0 });
  }, [isOpen, imageUrl, viewMode]);

  // Check if URL is from external domain
  const isExternalUrl = (url: string): boolean => {
    if (!url || url.startsWith('/') || url.startsWith('data:') || url.startsWith('blob:')) {
      return false;
    }
    try {
      const urlObj = new URL(url);
      // Check if it's from meituan.com, meituan.net, or sankuai.com domains
      const host = urlObj.hostname.toLowerCase();
      return (
        host.includes('meituan.com') ||
        host.includes('meituan.net') ||
        host.includes('sankuai.com')
      );
    } catch {
      return false;
    }
  };

  // Check if URL is a video based on extension
  const isVideoUrl = (url: string): boolean => {
    if (!url) return false;
    try {
      const urlObj = new URL(url);
      const pathname = urlObj.pathname.toLowerCase();
      const videoExtensions = ['.mp4', '.m3u8', '.webm', '.mov', '.avi', '.mkv', '.flv'];
      return videoExtensions.some(ext => pathname.endsWith(ext));
    } catch {
      // If URL parsing fails, try simple string check
      const lowerUrl = url.toLowerCase();
      return lowerUrl.includes('.mp4') || lowerUrl.includes('.m3u8') || 
             lowerUrl.includes('.webm') || lowerUrl.includes('.mov') ||
             lowerUrl.includes('.avi') || lowerUrl.includes('.mkv') || lowerUrl.includes('.flv');
    }
  };

  // Load proxied URL for external media
  useEffect(() => {
    let cancelled = false;
    if (isOpen && imageUrl) {
      setIsLoading(true);
      setProxiedUrl('');
      setImageSize({ width: 0, height: 0 });
      setViewMode('screen');
      setCustomScale(1);
      setMediaError(false);
      setDownloadError('');
      const videoType = isVideoUrl(imageUrl);
      setIsVideo(videoType);
      
      if (isExternalUrl(imageUrl)) {
        // Check if it's already a proxied URL
        if (imageUrl.includes('/media/image') || imageUrl.includes('/media/video')) {
          setProxiedUrl(imageUrl);
          return;
        }
        
        // Get proxied URL based on media type
        const getProxiedUrl = videoType ? mediaAPI.getVideoUrl : mediaAPI.getImageUrl;
        
        getProxiedUrl(imageUrl)
          .then((url) => {
            if (!cancelled) setProxiedUrl(url);
          })
          .catch((error) => {
            console.error('Failed to get proxied URL:', error);
            if (!cancelled) setProxiedUrl(imageUrl); // Fallback to original URL
          });
      } else {
        setProxiedUrl(imageUrl);
      }
    } else if (!isOpen) {
      // Reset when modal closes
      setProxiedUrl('');
      setIsLoading(true);
      setIsVideo(false);
    }
    return () => { cancelled = true; };
  }, [isOpen, imageUrl]);

  // Close on escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
      const previousOverflow = document.body.style.overflow;
      document.body.style.overflow = 'hidden';

      return () => {
        document.removeEventListener('keydown', handleEscape);
        document.body.style.overflow = previousOverflow;
      };
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isOpen, onClose]);

  // Pause video when modal closes
  useEffect(() => {
    if (!isOpen && videoRef.current) {
      videoRef.current.pause();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-[200] flex flex-col overflow-hidden overscroll-contain bg-black/80 backdrop-blur-sm"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={isVideo ? '视频预览' : '图片预览'}
    >
      <div className="flex min-h-0 flex-1 px-4 pb-4 pt-16 sm:px-8">
        <div ref={viewportRef} className="relative min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain">
          <div
            className="grid min-h-full min-w-full place-items-center"
            style={!isVideo && imageReady ? {
              width: Math.max(viewportSize.width, imageSize.width * scale),
              height: Math.max(viewportSize.height, imageSize.height * scale),
            } : undefined}
          >
          {/* Loading spinner */}
          {isLoading && (
            <div className="absolute inset-0 flex items-center justify-center" role="status" aria-label="正在加载">
              <div className="h-12 w-12 animate-spin rounded-full border-4 border-white border-t-transparent"></div>
            </div>
          )}

          {/* Video player */}
          {isVideo && proxiedUrl && (
            <video
              ref={videoRef}
              src={proxiedUrl}
              controls
              autoPlay
              onClick={(e) => e.stopPropagation()}
              className={`max-w-full rounded-lg ${isLoading ? 'hidden' : 'block'}`}
              style={{ maxHeight: viewportSize.height || undefined }}
              onLoadedData={() => setIsLoading(false)}
              onError={() => { setIsLoading(false); setMediaError(true); }}
            >
              Your browser does not support video playback.
            </video>
          )}

          {!isVideo && proxiedUrl && (
            <PreferenceImg
              key={proxiedUrl}
              src={proxiedUrl}
              alt="图片预览"
              draggable={false}
              onClick={(e) => e.stopPropagation()}
              className={`max-w-none rounded-lg ${imageReady ? 'block' : 'hidden'}`}
              style={{ width: imageSize.width * scale, height: imageSize.height * scale }}
              onLoad={(event) => {
                setImageSize({ width: event.currentTarget.naturalWidth, height: event.currentTarget.naturalHeight });
                setIsLoading(false);
              }}
              onError={() => { setIsLoading(false); setMediaError(true); }}
            />
          )}
          {mediaError && <p className="text-sm text-white" role="alert">{isVideo ? '视频' : '图片'}加载失败，请关闭后重试</p>}
          </div>
        </div>
      </div>
      {!isVideo && (
        <div className="shrink-0 border-t border-white/10 bg-black/65 px-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-3" onClick={(e) => e.stopPropagation()}>
          <div className="flex flex-wrap items-center justify-center gap-1 sm:gap-2" role="toolbar" aria-label="图片工具栏">
            <button type="button" className={toolbarButtonClass} disabled={!imageReady || scale <= 0.01} onClick={() => zoom(1 / 1.25)} aria-label="缩小图片" title="缩小">− <span className="hidden sm:inline">缩小</span></button>
            <output className="min-w-[4.5rem] text-center text-sm tabular-nums text-white" aria-label="当前缩放比例" aria-live="polite">{imageReady ? `${Math.round(scale * 1000) / 10}%` : '—'}</output>
            <button type="button" className={toolbarButtonClass} disabled={!imageReady || scale >= 16} onClick={() => zoom(1.25)} aria-label="放大图片" title="放大">＋ <span className="hidden sm:inline">放大</span></button>
            <span className="mx-1 h-5 w-px bg-white/20" aria-hidden="true" />
            <button type="button" className={toolbarButtonClass} disabled={!imageReady} onClick={() => { setCustomScale(1); setViewMode('custom'); }} title="按原始尺寸显示">100%</button>
            <button type="button" className={`${toolbarButtonClass} ${viewMode === 'screen' ? 'bg-white/15' : ''}`} disabled={!imageReady} aria-pressed={viewMode === 'screen'} onClick={() => setViewMode('screen')}>适配屏幕</button>
            <button type="button" className={`${toolbarButtonClass} ${viewMode === 'width' ? 'bg-white/15' : ''}`} disabled={!imageReady} aria-pressed={viewMode === 'width'} onClick={() => setViewMode('width')}>适配宽度</button>
            <span className="mx-1 hidden h-5 w-px bg-white/20 sm:block" aria-hidden="true" />
            <button type="button" className={toolbarButtonClass} disabled={!imageReady || isDownloading} onClick={downloadImage}>{isDownloading ? '下载中…' : '下载图片'}</button>
          </div>
          {downloadError && <p className="mt-2 text-center text-sm text-red-300" role="alert">{downloadError}</p>}
        </div>
      )}
      <button
        type="button"
        onClick={onClose}
        className="fixed right-4 top-4 z-[201] grid h-11 w-11 place-items-center rounded-full bg-black/45 text-white transition hover:bg-black/70 hover:text-gray-200 focus:outline-none focus:ring-2 focus:ring-white sm:right-6 sm:top-6"
        aria-label="关闭图片预览"
      >
        <svg className="h-7 w-7" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>,
    document.body
  );
}
