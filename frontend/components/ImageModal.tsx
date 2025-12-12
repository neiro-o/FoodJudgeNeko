'use client';

import { useEffect, useState, useRef } from 'react';
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
      document.body.style.overflow = 'hidden';
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
      document.body.style.overflow = 'unset';
    };
  }, [isOpen, onClose]);

  // Pause video when modal closes
  useEffect(() => {
    if (!isOpen && videoRef.current) {
      videoRef.current.pause();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-75"
      onClick={onClose}
    >
      <div className="relative max-w-[90vw] max-h-[90vh]">
        {/* Close button */}
        <button
          onClick={onClose}
          className="absolute -top-10 right-0 text-white hover:text-gray-300 transition"
        >
          <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {/* Loading spinner */}
        {isLoading && (
          <div className="flex items-center justify-center min-w-[200px] min-h-[200px]">
            <div className="animate-spin rounded-full h-12 w-12 border-4 border-white border-t-transparent"></div>
          </div>
        )}
        
        {/* Video player */}
        {isVideo && proxiedUrl && (
          <video
            ref={videoRef}
            src={proxiedUrl}
            controls
            autoPlay
            className={`max-w-full max-h-[85vh] rounded-lg ${isLoading ? 'hidden' : 'block'}`}
            onClick={(e) => e.stopPropagation()}
            onLoadedData={() => setIsLoading(false)}
            onError={() => setIsLoading(false)}
          >
            Your browser does not support video playback.
          </video>
        )}

        {/* Image - only render when we have a valid URL and it's not a video */}
        {!isVideo && proxiedUrl && (
          <img
            src={proxiedUrl}
            alt="Preview"
            className={`max-w-full max-h-[85vh] object-contain rounded-lg ${isLoading ? 'hidden' : 'block'}`}
            onClick={(e) => e.stopPropagation()}
            onLoad={() => setIsLoading(false)}
            onError={() => setIsLoading(false)}
          />
        )}
      </div>
    </div>
  );
}
