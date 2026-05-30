'use client';

import { useState } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';
import ImageModal from './ImageModal';

export interface ChatMessageData {
  role: 'user' | 'merchant' | string; // string for 'others_{id}'
  name: string;
  timestamp: number;
  content: string;
  pics?: string[];
}

interface ChatMessageProps {
  message: ChatMessageData;
  userAvatar?: string;
  merchantAvatar?: string;
}

export default function ChatMessage({ 
  message, 
  userAvatar = '/avatars/avatar_1.png',
  merchantAvatar = '/avatars/avatar_2.png'
}: ChatMessageProps) {
  const { t, language } = useLanguage();
  const { isDark } = useTheme();
  const [modalImage, setModalImage] = useState<string | null>(null);

  // Determine if message is on the right (merchant)
  const isRight = message.role === 'merchant';
  
  // Format timestamp
  const formatTime = (timestamp: number): string => {
    const date = new Date(timestamp * 1000);
    const year = date.getFullYear();
    const month = date.getMonth() + 1;
    const day = date.getDate();
    const hours = date.getHours();
    const minutes = date.getMinutes().toString().padStart(2, '0');
    const seconds = date.getSeconds().toString().padStart(2, '0');
    return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
  };

  // Get avatar based on role
  const getAvatar = () => {
    if (message.role === 'merchant') {
      return merchantAvatar;
    }
    if (message.role === 'user') {
      return userAvatar;
    }
    // For any other role (not user or merchant), use avatar_3.png
    return '/avatars/avatar_3.png';
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

  // Get media label (picture or video)
  const getMediaLabel = (url: string, index: number) => {
    if (isVideoUrl(url)) {
      return language === 'zh' ? `视频${index + 1}` : `Video ${index + 1}`;
    }
    return language === 'zh' ? `图片${index + 1}` : `Picture ${index + 1}`;
  };

  return (
    <>
      <div className={`flex gap-3 mb-4 ${isRight ? 'flex-row-reverse' : 'flex-row'}`}>
        {/* Avatar */}
        <div className="flex-shrink-0">
          <img
            src={getAvatar()}
            alt={message.name}
            className="w-10 h-10 rounded-full object-cover"
          />
        </div>

        {/* Message content */}
        <div className={`flex flex-col ${isRight ? 'items-end' : 'items-start'} max-w-[70%]`}>
          {/* Name and time */}
          <div className={`text-xs text-gray-500 mb-1 ${isRight ? 'text-right' : 'text-left'}`}>
            {message.name} ({formatTime(message.timestamp)})
          </div>

          {/* Message bubble */}
          <div
            className={`px-4 py-2 rounded-lg text-gray-900 dark:text-gray-100 ${
              message.role === 'merchant'
                ? 'rounded-tr-none'
                : 'rounded-tl-none'
            }`}
            style={{
              backgroundColor:
                message.role === 'user'
                  ? (isDark ? '#5a3a1a' : '#ffdfbf')
                  : message.role === 'merchant'
                  ? (isDark ? '#0c3a52' : '#bce6ff')
                  : (isDark ? '#374151' : '#f3f4f6'),
            }}
          >
            <p className="whitespace-pre-wrap break-words">{message.content}</p>
            
            {/* Picture/Video links */}
            {message.pics && message.pics.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-2">
                {message.pics.map((pic, index) => (
                  <button
                    key={index}
                    onClick={() => setModalImage(pic)}
                    className="text-indigo-600 hover:text-indigo-800 hover:underline text-sm"
                  >
                    {getMediaLabel(pic, index)}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Image Modal */}
      <ImageModal
        imageUrl={modalImage || ''}
        isOpen={!!modalImage}
        onClose={() => setModalImage(null)}
      />
    </>
  );
}
