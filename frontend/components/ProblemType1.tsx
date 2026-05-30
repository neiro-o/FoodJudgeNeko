'use client';

import { useState, useEffect, useMemo } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
import ChatTimeline from './ChatTimeline';
import { ChatMessageData } from './ChatMessage';
import ImageModal from './ImageModal';
import RatioBar from './RatioBar';
import { mediaAPI } from '@/lib/api';

interface Reply {
  role: string;
  timestamp: number;
  content: string;
}

interface Appeal {
  role: string;
  timestamp: number;
  content: string;
  pics?: string[];
}

interface Order {
  name: string;
  count: number;
  desc: string;
  selection: string[];
  pic: string;
  others: string;
}

interface OrderDetail {
  order_started: number;
  order_finished: number;
  deliver_time: number;
  total_time: number;
  deliver_by: string;
  note: string;
  utensils: number;
  invoice: boolean;
}

interface ProblemType1Props {
  userReview: string;
  reviewPics?: string[];
  timestamp: number;
  replies?: Reply[];
  appeals?: Appeal[];
  orders?: Order[];
  orderDetail?: OrderDetail;
  others?: string;
  ratio1?: number;
  ratio2?: number;
  answer?: number;
}

export default function ProblemType1({
  userReview,
  reviewPics,
  timestamp,
  replies = [],
  appeals = [],
  orders = [],
  orderDetail,
  others,
  ratio1 = 50,
  ratio2 = 50,
  answer = 1,
}: ProblemType1Props) {
  const { language } = useLanguage();
  const [expandedDescs, setExpandedDescs] = useState<Set<number>>(new Set());
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

  // Get proxied media URL for external URLs (image or video)
  const getMediaUrl = async (originalUrl: string): Promise<string> => {
    if (!isExternalUrl(originalUrl)) {
      return originalUrl;
    }

    // Check cache first
    if (imageUrlMap.has(originalUrl)) {
      return imageUrlMap.get(originalUrl)!;
    }

    try {
      const getProxiedUrl = isVideoUrl(originalUrl) ? mediaAPI.getVideoUrl : mediaAPI.getImageUrl;
      const proxiedUrl = await getProxiedUrl(originalUrl);
      setImageUrlMap((prev) => new Map(prev).set(originalUrl, proxiedUrl));
      return proxiedUrl;
    } catch (error) {
      console.error('Failed to get proxied URL:', error);
      return originalUrl; // Fallback to original URL
    }
  };

  // Preload external media (images and videos)
  useEffect(() => {
    const preloadMedia = async () => {
      const mediaUrls: string[] = [];

      // Collect all media URLs
      orders.forEach((order) => {
        if (order.pic && isExternalUrl(order.pic)) {
          mediaUrls.push(order.pic);
        }
      });

      if (reviewPics) {
        reviewPics.forEach((pic) => {
          if (isExternalUrl(pic)) {
            mediaUrls.push(pic);
          }
        });
      }

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
  }, [orders, reviewPics, appeals]);

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

  // Build messages array for problem section
  const buildProblemMessages = (): ChatMessageData[] => {
    const messages: ChatMessageData[] = [];

    // First message is always from user
    // Proxy image URLs if they are external
    const proxiedReviewPics = reviewPics?.map((pic) => 
      imageUrlMap.get(pic) || pic
    );

    messages.push({
      role: 'user',
      name: getRoleName('user'),
      timestamp: timestamp,
      content: userReview,
      pics: proxiedReviewPics,
    });

    // Add replies
    replies.forEach((reply) => {
      messages.push({
        role: reply.role,
        name: getRoleName(reply.role),
        timestamp: reply.timestamp,
        content: reply.content,
      });
    });

    return messages;
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

  // Format timestamp to GMT+8
  const formatTimestamp = (ts: number): string => {
    const date = new Date(ts * 1000);
    // Force GMT+8
    const utc = date.getTime() + date.getTimezoneOffset() * 60000;
    const gmt8 = new Date(utc + 8 * 3600000);
    const year = gmt8.getFullYear();
    const month = (gmt8.getMonth() + 1).toString().padStart(2, '0');
    const day = gmt8.getDate().toString().padStart(2, '0');
    const hours = gmt8.getHours().toString().padStart(2, '0');
    const minutes = gmt8.getMinutes().toString().padStart(2, '0');
    const seconds = gmt8.getSeconds().toString().padStart(2, '0');
    return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
  };

  // Format seconds to m:ss
  const formatDuration = (seconds: number): string => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  // Get deliver_by display text
  const getDeliverByText = (deliverBy: string): string => {
    if (deliverBy === 'meituan') {
      return language === 'zh' ? '美团配送' : 'Meituan';
    } else if (deliverBy === 'merchant') {
      return language === 'zh' ? '商家配送' : 'Merchant';
    } else if (deliverBy === 'user') {
      return language === 'zh' ? '用户自取' : 'Self Pickup';
    }
    return deliverBy;
  };

  // Toggle description expansion
  const toggleDesc = (index: number) => {
    const newExpanded = new Set(expandedDescs);
    if (newExpanded.has(index)) {
      newExpanded.delete(index);
    } else {
      newExpanded.add(index);
    }
    setExpandedDescs(newExpanded);
  };

  // Calculate total items count
  const totalItemsCount = orders.reduce((sum, order) => sum + order.count, 0);

  // Rebuild messages when imageUrlMap changes
  const problemMessages = useMemo(() => buildProblemMessages(), [reviewPics, replies, timestamp, imageUrlMap, language]);
  const appealMessages = useMemo(() => buildAppealMessages(), [appeals, imageUrlMap, language]);
  
  const problemTitle = language === 'zh' ? '题目' : 'Problem';
  const problemDescription = language === 'zh' ? '外卖评价投诉纠纷' : 'Takeaway Review Disputes';
  const appealsTitle = language === 'zh' ? '商户申诉' : 'Appeals';
  const othersTitle = language === 'zh' ? '其他信息' : 'Other Info';
  const ordersTitle = language === 'zh' ? '订单信息' : 'Orders';
  const notesSubtitle = language === 'zh' ? '用户备注' : 'Notes';
  const itemsSubtitle = language === 'zh' ? `商品 (${totalItemsCount} 件)` : `Items (${totalItemsCount})`;
  const deliverySubtitle = language === 'zh' ? '配送信息' : 'Delivery';

  return (
    <div>
      {/* Problem Section */}
      <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">{problemTitle}</h2>
      <RatioBar ratio1={ratio1} ratio2={ratio2} answer={answer} />
      <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">{problemDescription}</h3>
      <div className="mb-6">
        <ChatTimeline messages={problemMessages} />
      </div>

      {/* Appeals Section */}
      {appealMessages.length > 0 && (
        <>
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">{appealsTitle}</h3>
          <div className="mb-6">
            <ChatTimeline messages={appealMessages} />
          </div>
        </>
      )}

      {/* Others Section */}
      {others && (
        <div className="border-t border-gray-200 dark:border-gray-700 pt-4 mt-4 mb-6">
          <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">{othersTitle}</h3>
          <p className="text-sm text-gray-600 dark:text-gray-400 whitespace-pre-wrap">{others}</p>
        </div>
      )}

      {/* Orders Section */}
      {orderDetail && (
        <>
          <h3 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">{ordersTitle}</h3>
          
          {/* Notes Subtitle */}
          <div className="flex flex-wrap gap-2 mb-2">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{notesSubtitle}</h4>
          
          {/* Tags */}
            <span>
            {/* Invoice Tag */}
            {orderDetail.invoice ? (
              <span className="inline-flex items-center px-2.5 py-1 rounded text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
                {language === 'zh' ? '开具发票' : 'Need Invoice'}
              </span>
            ) : (
              <span className="inline-flex items-center px-2.5 py-1 rounded text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300">
                {language === 'zh' ? '不开发票' : 'No Invoice'}
              </span>
            )}
            
            {/* Utensils Tag */}
            {orderDetail.utensils === 0 ? (
              <span className="inline-flex items-center px-2.5 py-1 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200">
                {language === 'zh' ? '无需餐具（环保单）' : 'No utensil'}
              </span>
            ) : orderDetail.utensils === -1 ? (
              <span className="inline-flex items-center px-2.5 py-1 rounded text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
                {language === 'zh' ? '需要餐具' : 'Need utensils'}
              </span>
            ) : (
              <span className="inline-flex items-center px-2.5 py-1 rounded text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
                {language === 'zh' ? `需要餐具×${orderDetail.utensils}` : `Need utensils × ${orderDetail.utensils}`}
              </span>
            )}
            </span>
          </div>
          
          {/* Note Content */}
          {orderDetail.note && (
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">{orderDetail.note}</p>
          )}
          
          {/* Two Column Layout */}
          <div className="flex flex-col lg:flex-row">
            {/* Left: Items (2/3) */}
            <div className="w-full lg:w-2/3 lg:pr-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{itemsSubtitle}</h4>
              <div className="space-y-3">
                {orders.map((order, index) => (
                  <div key={index} className="flex gap-3">
                    {/* Product Image */}
                    <div className="flex-shrink-0">
                      {isExternalUrl(order.pic) && !imageUrlMap.has(order.pic) ? (
                        // Show placeholder while loading proxied URL
                        <div className="w-16 h-16 bg-gray-200 dark:bg-gray-700 rounded flex items-center justify-center">
                          <div className="animate-spin rounded-full h-6 w-6 border-2 border-gray-400 border-t-transparent"></div>
                        </div>
                      ) : (
                        <img
                          src={imageUrlMap.get(order.pic) || order.pic}
                          alt={order.name}
                          className="w-16 h-16 object-cover rounded cursor-pointer hover:opacity-80 transition"
                          onClick={() => {
                            const imageUrl = imageUrlMap.get(order.pic) || order.pic;
                            setModalImage(imageUrl);
                          }}
                        />
                      )}
                    </div>
                    
                    {/* Product Info */}
                    <div className="flex-1 min-w-0">
                      {/* Name */}
                      <p className="text-sm font-medium text-gray-900 dark:text-gray-100 break-words">
                        {order.name} {order.count > 1 && `×${order.count}`}
                      </p>
                      
                      {/* Selection */}
                      {order.selection && order.selection.length > 0 && (
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          {order.selection.join('; ')}
                        </p>
                      )}
                      
                      {/* Others */}
                      {order.others && (
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{order.others}</p>
                      )}
                      
                      {/* Desc (expandable) */}
                      {order.desc && (
                        <button
                          onClick={() => toggleDesc(index)}
                          className={`mt-1 text-xs bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-600 dark:text-gray-300 px-2 py-1 rounded transition ${
                            expandedDescs.has(index) ? 'text-left' : ''
                          }`}
                        >
                          {expandedDescs.has(index) ? (
                            <span className="block text-left">{order.desc}</span>
                          ) : (
                            <span>{language === 'zh' ? '查看详情' : 'View details'}</span>
                          )}
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
            
            {/* Right: Delivery Info (1/3) */}
            <div className="w-full lg:w-1/3 mt-4 lg:mt-0">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{deliverySubtitle}</h4>
              <div className="space-y-1 text-sm">
                {/* Deliver By */}
                <p className="text-gray-600 dark:text-gray-400">
                  <span className="text-gray-500 dark:text-gray-500">{language === 'zh' ? '配送方式：' : 'Delivered by: '}</span>
                  <span className="inline-flex items-center px-2.5 py-1 rounded text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300">{getDeliverByText(orderDetail.deliver_by)}</span>
                </p>
                
                {/* Order Started */}
                <p className="text-gray-600 dark:text-gray-400">
                  <span className="text-gray-500 dark:text-gray-500">{language === 'zh' ? '下单时间：' : 'Ordered at: '}</span>
                  {formatTimestamp(orderDetail.order_started)}
                </p>
                
                {/* Order Finished (only if > 0) */}
                {orderDetail.order_finished > 0 && (
                  <p className="text-gray-600 dark:text-gray-400">
                    <span className="text-gray-500 dark:text-gray-500">{language === 'zh' ? '送达时间：' : 'Finished at: '}</span>
                    {formatTimestamp(orderDetail.order_finished)}
                  </p>
                )}
                
                {/* Deliver Time (only if meituan) */}
                {orderDetail.deliver_by === 'meituan' && orderDetail.deliver_time > 0 && (
                  <p className="text-gray-600 dark:text-gray-400">
                    <span className="text-gray-500 dark:text-gray-500">{language === 'zh' ? '配送时长：' : 'Delivery time: '}</span>
                    {formatDuration(orderDetail.deliver_time)}
                  </p>
                )}
                
                {/* Total Time (not for user/self pickup) */}
                {orderDetail.deliver_by !== 'user' && orderDetail.total_time > 0 && (
                  <p className="text-gray-600 dark:text-gray-400">
                    <span className="text-gray-500 dark:text-gray-500">{language === 'zh' ? '总时长：' : 'Total time: '}</span>
                    {formatDuration(orderDetail.total_time)}
                  </p>
                )}
              </div>
            </div>
          </div>
        </>
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
