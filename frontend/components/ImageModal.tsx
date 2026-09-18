'use client';

import { useEffect, useState, useRef } from 'react';
import { createPortal } from 'react-dom';
import { mediaAPI } from '@/lib/api';

interface ImageModalProps {
  imageUrl: string;
  isOpen: boolean;
  onClose: () => void;
}

export default function ImageModal({ imageUrl, isOpen, onClose }: ImageModalProps) {
  const [isLoading, setIsLoading] = useState(true);
  const [proxiedUrl, setProxiedUrl] = useState<string>('');
  const [isVideo, setIsVideo] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);

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
    if (isOpen && imageUrl) {
      setIsLoading(true);
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
            setProxiedUrl(url);
          })
          .catch((error) => {
            console.error('Failed to get proxied URL:', error);
            setProxiedUrl(imageUrl); // Fallback to original URL
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
      className="fixed inset-0 z-[200] overflow-y-auto overscroll-contain bg-black/80 backdrop-blur-sm"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="图片预览"
    >
      {/* Keep the media centered when it fits, while allowing tall images to scroll. */}
      <div className="grid min-h-full place-items-center px-4 py-16 sm:px-8">
        <div className="relative max-w-[92vw]" onClick={(e) => e.stopPropagation()}>
          {/* Loading spinner */}
          {isLoading && (
            <div className="flex min-h-[200px] min-w-[200px] items-center justify-center">
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
              className={`max-h-[calc(100svh-8rem)] max-w-full rounded-lg ${isLoading ? 'hidden' : 'block'}`}
              onLoadedData={() => setIsLoading(false)}
              onError={() => setIsLoading(false)}
            >
              Your browser does not support video playback.
            </video>
          )}

          {/* Preserve tall images at a readable size; the overlay provides scrolling. */}
          {!isVideo && proxiedUrl && (
            <img
              src={proxiedUrl}
              alt="Preview"
              className={`h-auto max-w-full rounded-lg object-contain ${isLoading ? 'hidden' : 'block'}`}
              onLoad={() => setIsLoading(false)}
              onError={() => setIsLoading(false)}
            />
          )}
        </div>
      </div>
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
