'use client';

import { useEffect, useState } from 'react';
import { mediaAPI } from '@/lib/api';

interface ImageModalProps {
  imageUrl: string;
  isOpen: boolean;
  onClose: () => void;
}

export default function ImageModal({ imageUrl, isOpen, onClose }: ImageModalProps) {
  const [isLoading, setIsLoading] = useState(true);
  const [proxiedImageUrl, setProxiedImageUrl] = useState<string>(imageUrl);

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

  // Load proxied URL for external images
  useEffect(() => {
    if (isOpen && imageUrl) {
      setIsLoading(true);
      
      if (isExternalUrl(imageUrl)) {
        // Check if it's already a proxied URL (contains /media/image endpoint)
        if (imageUrl.includes('/media/image')) {
          setProxiedImageUrl(imageUrl);
          // Keep loading true - will be set to false when image loads via onLoad handler
          return;
        }
        
        // Get proxied URL
        mediaAPI.getImageUrl(imageUrl)
          .then((proxiedUrl) => {
            setProxiedImageUrl(proxiedUrl);
            // Keep loading true - will be set to false when image loads via onLoad handler
          })
          .catch((error) => {
            console.error('Failed to get proxied image URL:', error);
            setProxiedImageUrl(imageUrl); // Fallback to original URL
            // Keep loading true - will be set to false when image loads via onLoad handler
          });
      } else {
        setProxiedImageUrl(imageUrl);
        // Keep loading true - will be set to false when image loads via onLoad handler
      }
    } else if (!isOpen) {
      // Reset when modal closes
      setProxiedImageUrl('');
      setIsLoading(true); // Reset loading state for next open
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
        
        {/* Image - only render when we have a valid URL */}
        {proxiedImageUrl && (
          <img
            src={proxiedImageUrl}
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
