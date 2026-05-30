'use client';

import { useState, useEffect, useMemo } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
import ChatTimeline from './ChatTimeline';
import { ChatMessageData } from './ChatMessage';
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

interface ProblemType2Props {
  userReview: string;
  reviewPics?: string[];
  timestamp: number;
  replies?: Reply[];
  appeals?: Appeal[];
  others?: string;
  ratio1?: number;
  ratio2?: number;
  answer?: number;
}

export default function ProblemType2({
  userReview,
  reviewPics,
  timestamp,
  replies = [],
  appeals = [],
  others,
  ratio1 = 50,
  ratio2 = 50,
  answer = 1,
}: ProblemType2Props) {
  const { language } = useLanguage();
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
  }, [reviewPics, appeals]);

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

  // Rebuild messages when imageUrlMap changes
  const problemMessages = useMemo(() => buildProblemMessages(), [reviewPics, replies, timestamp, imageUrlMap, language]);
  const appealMessages = useMemo(() => buildAppealMessages(), [appeals, imageUrlMap, language]);
  
  const problemTitle = language === 'zh' ? '题目' : 'Problem';
  const problemDescription = language === 'zh' ? '堂食评价纠纷' : 'Dining Review Disputes';
  const appealsTitle = language === 'zh' ? '商户申诉' : 'Appeals';
  const othersTitle = language === 'zh' ? '其他信息' : 'Other Info';

  // Import RatioBar dynamically to avoid issues
  const RatioBar = require('./RatioBar').default;

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
        <div className="border-t border-gray-200 dark:border-gray-700 pt-4 mt-4">
          <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">{othersTitle}</h3>
          <p className="text-sm text-gray-600 dark:text-gray-400 whitespace-pre-wrap">{others}</p>
        </div>
      )}
    </div>
  );
}
