'use client';

import { useState, useEffect, useMemo } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
import ChatTimeline from './ChatTimeline';
import { ChatMessageData } from './ChatMessage';
import ImageModal from './ImageModal';
import RatioBar from './RatioBar';
import { mediaAPI } from '@/lib/api';

interface Appeal {
  role: string;
  timestamp: number;
  content: string;
  pics?: string[];
}

interface OrderInfoRaw {
  productImageUrl?: string;
  productName?: string;
  orderPrice?: number;
  [key: string]: any;
}

interface OrderInfo {
  raw?: OrderInfoRaw;
  processed?: any;
}

interface ProblemType4Props {
  appeals?: Appeal[];
  orderInfo?: OrderInfo;
  others?: string;
  ratio1?: number;
  ratio2?: number;
  answer?: number;
}

export default function ProblemType4({
  appeals = [],
  orderInfo,
  others,
  ratio1 = 50,
  ratio2 = 50,
  answer = 1,
}: ProblemType4Props) {
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
      const lowerUrl = url.toLowerCase();
      return lowerUrl.includes('.mp4') || lowerUrl.includes('.m3u8') || 
             lowerUrl.includes('.webm') || lowerUrl.includes('.mov') ||
             lowerUrl.includes('.avi') || lowerUrl.includes('.mkv') || lowerUrl.includes('.flv');
    }
  };

  // Preload external media (images and videos)
  useEffect(() => {
    const preloadMedia = async () => {
      const mediaUrls: string[] = [];

      // Collect product image URL
      if (orderInfo?.raw?.productImageUrl && isExternalUrl(orderInfo.raw.productImageUrl)) {
        mediaUrls.push(orderInfo.raw.productImageUrl);
      }

      // Collect appeal pics
      appeals.forEach((appeal) => {
        if (appeal.pics) {
          appeal.pics.forEach((pic) => {
            if (isExternalUrl(pic)) {
              mediaUrls.push(pic);
            }
          });
        }
      });

      // Preload all external media
      const uniqueUrls = Array.from(new Set(mediaUrls));
      for (const url of uniqueUrls) {
        try {
          const getProxiedUrl = isVideoUrl(url) ? mediaAPI.getVideoUrl : mediaAPI.getImageUrl;
          const proxiedUrl = await getProxiedUrl(url);
          setImageUrlMap((prev) => new Map(prev).set(url, proxiedUrl));
        } catch (error) {
          console.error('Failed to preload media:', url, error);
        }
      }
    };

    preloadMedia();
  }, [orderInfo, appeals]);

  // Get role name based on role and language
  const getRoleName = (role: string): string => {
    if (role === 'merchant') {
      return language === 'zh' ? '商户' : 'Merchant';
    } else if (role === 'user') {
      return language === 'zh' ? '用户' : 'User';
    } else if (role.startsWith('others_')) {
      const id = role.replace('others_', '');
      return language === 'zh' ? `路人${id}` : `Anonymous ${id}`;
    }
    return role;
  };

  // Build messages array for appeals section
  const buildAppealMessages = (): ChatMessageData[] => {
    return appeals.map((appeal) => {
      // Proxy image URLs if they are external
      const proxiedPics = appeal.pics?.map((pic) => 
        imageUrlMap.get(pic) || pic
      );
      
      return {
        role: appeal.role,
        name: getRoleName(appeal.role),
        timestamp: appeal.timestamp,
        content: appeal.content,
        pics: proxiedPics,
      };
    });
  };

  // Rebuild messages when imageUrlMap changes
  const appealMessages = useMemo(() => buildAppealMessages(), [appeals, imageUrlMap, language]);
  
  // Check if order info should be shown (only if productName AND orderPrice exist)
  const showOrderInfo = orderInfo?.raw?.productName && orderInfo?.raw?.orderPrice !== undefined;
  
  // Get product image URL (proxied if external)
  const productImageUrl = orderInfo?.raw?.productImageUrl 
    ? (imageUrlMap.get(orderInfo.raw.productImageUrl) || orderInfo.raw.productImageUrl)
    : null;
  
  const problemTitle = language === 'zh' ? '题目' : 'Problem';
  const problemDescription = language === 'zh' ? '线下服务退款申诉' : 'Offline Service Refund Appeal';
  const ordersTitle = language === 'zh' ? '订单信息' : 'Orders';
  const noteTitle = language === 'zh' ? '备注' : 'Note';

  return (
    <div>
      {/* Problem Section */}
      <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">{problemTitle}</h2>
      <RatioBar ratio1={ratio1} ratio2={ratio2} answer={answer} />
      <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">{problemDescription}</h3>

      {/* Order Info Section */}
      {showOrderInfo && (
        <div className="mb-6">
          <h3 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">{ordersTitle}</h3>
          <div className="flex gap-4">
            {/* Product Image */}
            {productImageUrl && (
              <div className="flex-shrink-0">
                {isExternalUrl(orderInfo?.raw?.productImageUrl || '') && !imageUrlMap.has(orderInfo?.raw?.productImageUrl || '') ? (
                  // Show placeholder while loading proxied URL
                  <div className="w-20 h-20 bg-gray-200 dark:bg-gray-700 rounded flex items-center justify-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-2 border-gray-400 border-t-transparent"></div>
                  </div>
                ) : (
                  <img
                    src={productImageUrl}
                    alt={orderInfo?.raw?.productName || 'Product'}
                    className="w-20 h-20 object-cover rounded cursor-pointer hover:opacity-80 transition"
                    onClick={() => setModalImage(productImageUrl)}
                  />
                )}
              </div>
            )}
            
            {/* Product Info */}
            <div className="flex-1 min-w-0">
              {/* Product Name */}
              <p className="text-sm font-medium text-gray-900 dark:text-gray-100 break-words">
                {orderInfo?.raw?.productName}
              </p>
              
              {/* Price */}
              <p className="text-sm text-orange-600 font-medium mt-1">
                ¥{orderInfo?.raw?.orderPrice}
              </p>
            </div>
          </div>
          
          {/* Others as Note */}
          {others && (
            <div className="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                <span className="font-medium">{noteTitle}: </span>
                <span className="text-gray-600 dark:text-gray-300">{others}</span>
              </p>
            </div>
          )}
        </div>
      )}

      {/* Appeals Section */}
      {appealMessages.length > 0 && (
        <div className="mb-6">
          <ChatTimeline messages={appealMessages} />
        </div>
      )}

      {/* Image Modal */}
      <ImageModal
        imageUrl={modalImage ? (imageUrlMap.get(modalImage) || modalImage) : ''}
        isOpen={!!modalImage}
        onClose={() => setModalImage(null)}
      />
    </div>
  );
}
