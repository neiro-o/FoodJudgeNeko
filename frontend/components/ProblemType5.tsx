'use client';

import { useState, useEffect } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
import ImageModal from './ImageModal';
import RatioBar from './RatioBar';
import { mediaAPI } from '@/lib/api';

interface ProblemType5Props {
  userReview: string;
  reviewPics?: string[];
  timestamp: number;
  ratio1?: number;
  ratio2?: number;
  answer?: number;
}

export default function ProblemType5({
  userReview,
  reviewPics = [],
  timestamp,
  ratio1 = 50,
  ratio2 = 50,
  answer = 1,
}: ProblemType5Props) {
  const { language } = useLanguage();
  const [modalImage, setModalImage] = useState<string | null>(null);
  const [imageUrlMap, setImageUrlMap] = useState<Map<string, string>>(new Map());

  // Check if URL is from external domain (not same origin)
  const isExternalUrl = (url: string): boolean => {
    if (!url || url.startsWith('/') || url.startsWith('data:')) {
      return false;
    }
    try {
      const urlObj = new URL(url);
      // Check if it's from meituan.com, meituan.net, or sankuai.com domains
      const host = urlObj.hostname.toLowerCase();
      return (
        host.includes('meituan.com') ||
        host.includes('meituan.net') ||
        host.includes('sankuai.com') ||
        host.includes('s3plus.meituan.net')
      );
    } catch {
      return false;
    }
  };

  // Preload external images
  useEffect(() => {
    const preloadMedia = async () => {
      const mediaUrls: string[] = [];

      reviewPics.forEach((pic) => {
        if (isExternalUrl(pic)) {
          mediaUrls.push(pic);
        }
      });

      // Preload all external media
      const uniqueUrls = Array.from(new Set(mediaUrls));
      for (const url of uniqueUrls) {
        try {
          const proxiedUrl = await mediaAPI.getImageUrl(url);
          setImageUrlMap((prev) => new Map(prev).set(url, proxiedUrl));
        } catch (error) {
          console.error('Failed to preload media:', url, error);
        }
      }
    };

    preloadMedia();
  }, [reviewPics]);

  // Format timestamp to GMT+8
  const formatTimestamp = (ts: number): string => {
    const date = new Date(ts * 1000);
    // Force GMT+8
    const utc = date.getTime() + date.getTimezoneOffset() * 60000;
    const gmt8 = new Date(utc + 8 * 3600000);
    const year = gmt8.getFullYear();
    const month = gmt8.getMonth() + 1;
    const day = gmt8.getDate();
    const hours = gmt8.getHours();
    const minutes = gmt8.getMinutes().toString().padStart(2, '0');
    const seconds = gmt8.getSeconds().toString().padStart(2, '0');
    return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
  };

  // Get image URL (proxied if external)
  const getImageUrl = (url: string): string => {
    return imageUrlMap.get(url) || url;
  };

  const problemTitle = language === 'zh' ? '题目' : 'Problem';
  const problemDescription = language === 'zh' ? '商标争议' : 'Trademark Problem';
  const hintText = language === 'zh' 
    ? '判断商标是否容易混淆，混淆选"适合展示"' 
    : 'Support User if 2 trademarks are easy to be confused.';
  const infoTitle = language === 'zh' ? '补充信息' : 'Additional Info';
  const disputeLabel = language === 'zh' ? '争议内容' : 'Dispute content';
  const startTimeLabel = language === 'zh' ? '发起时间' : 'Start Time';

  return (
    <div>
      {/* Problem Section */}
      <h2 className="text-lg font-semibold text-gray-900 mb-4">{problemTitle}</h2>
      <RatioBar ratio1={ratio1} ratio2={ratio2} answer={answer} />
      <h3 className="text-lg font-semibold text-gray-900 mb-2">{problemDescription}</h3>
      
      {/* Hint */}
      <div className="mb-4 p-3 bg-amber-50 border border-amber-200 rounded-lg">
        <p className="text-sm text-amber-800">
          <span className="font-medium">💡 </span>
          {hintText}
        </p>
      </div>

      {/* Two Trademark Images */}
      {reviewPics.length > 0 && (
        <div className="mb-6">
          <div className="flex flex-row gap-4 justify-center">
            {reviewPics.map((pic, index) => (
              <div key={index} className="flex-1 max-w-[45%]">
                {isExternalUrl(pic) && !imageUrlMap.has(pic) ? (
                  // Show placeholder while loading
                  <div className="w-full aspect-square bg-gray-200 rounded-lg flex items-center justify-center">
                    <div className="animate-spin rounded-full h-8 w-8 border-2 border-gray-400 border-t-transparent"></div>
                  </div>
                ) : (
                  <img
                    src={getImageUrl(pic)}
                    alt={`Trademark ${index + 1}`}
                    className="w-full aspect-square object-contain rounded-lg border border-gray-200 bg-white cursor-pointer hover:opacity-90 transition"
                    onClick={() => setModalImage(pic)}
                  />
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Info Section */}
      <div className="bg-gray-50 rounded-lg p-4">
        <h4 className="text-sm font-semibold text-gray-900 mb-3">{infoTitle}</h4>
        <div className="space-y-2 text-sm">
          {/* Dispute Content */}
          <p className="text-gray-700">
            <span className="text-gray-500 font-medium">{disputeLabel}: </span>
            {userReview}
          </p>
          
          {/* Start Time */}
          <p className="text-gray-700">
            <span className="text-gray-500 font-medium">{startTimeLabel}: </span>
            {formatTimestamp(timestamp)}
          </p>
        </div>
      </div>

      {/* Image Modal */}
      <ImageModal
        imageUrl={modalImage ? getImageUrl(modalImage) : ''}
        isOpen={!!modalImage}
        onClose={() => setModalImage(null)}
      />
    </div>
  );
}
