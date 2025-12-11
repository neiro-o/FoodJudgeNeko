'use client';

import { useState } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
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
    return userAvatar;
  };

  // Get picture label
  const getPictureLabel = (index: number) => {
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
            className={`px-4 py-2 rounded-lg text-gray-900 ${
              message.role === 'merchant'
                ? 'rounded-tr-none'
                : 'rounded-tl-none'
            }`}
            style={{
              backgroundColor:
                message.role === 'user'
                  ? '#ffdfbf'
                  : message.role === 'merchant'
                  ? '#bce6ff'
                  : '#f3f4f6',
            }}
          >
            <p className="whitespace-pre-wrap break-words">{message.content}</p>
            
            {/* Picture links */}
            {message.pics && message.pics.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-2">
                {message.pics.map((pic, index) => (
                  <button
                    key={index}
                    onClick={() => setModalImage(pic)}
                    className="text-indigo-600 hover:text-indigo-800 hover:underline text-sm"
                  >
                    {getPictureLabel(index)}
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
