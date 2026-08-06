'use client';

import { useLanguage } from '@/contexts/LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';
import AccountAvatar from './AccountAvatar';

interface RatioBarProps {
  ratio1: number;
  ratio2: number;
  answer: number; // 1 = support user, 2 = support merchant
  uploaders?: UploaderAccount[];
}

export interface UploaderAccount {
  id: string;
  name: string;
}

export default function RatioBar({ ratio1, ratio2, answer, uploaders = [] }: RatioBarProps) {
  const { language } = useLanguage();
  const { isDark } = useTheme();

  // Colors based on answer
  const leftColor = answer === 1 ? (isDark ? '#5a3a1a' : '#ffdfbf') : (isDark ? '#374151' : '#f3f4f6');
  const rightColor = answer === 2 ? (isDark ? '#0c3a52' : '#bce6ff') : (isDark ? '#374151' : '#f3f4f6');
  const leftTextColor = answer === 1 ? (isDark ? '#fbbf24' : '#d97706') : (isDark ? '#6b7280' : '#9ca3af');
  const rightTextColor = answer === 2 ? (isDark ? '#38bdf8' : '#0284c7') : (isDark ? '#6b7280' : '#9ca3af');

  // Labels
  const leftLabel = language === 'zh' ? '适合展示' : 'Support User';
  const rightLabel = language === 'zh' ? '不适合展示' : 'Support Merchant';

  return (
    <div className="mb-4">
      {/* Bar */}
      <div className="flex h-8 rounded-lg overflow-hidden">
        {/* Left portion */}
        <div
          className="flex items-center justify-start px-3"
          style={{
            width: `${ratio1}%`,
            backgroundColor: leftColor,
          }}
        >
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            {Math.round(ratio1)}%
          </span>
        </div>
        
        {/* Right portion */}
        <div
          className="flex items-center justify-end px-3"
          style={{
            width: `${ratio2}%`,
            backgroundColor: rightColor,
          }}
        >
          <span className="text-sm font-medium text-gray-500 dark:text-gray-300">
            {Math.round(ratio2)}%
          </span>
        </div>
      </div>

      {/* Labels */}
      <div className="flex justify-between mt-1">
        <span
          className="text-sm font-medium"
          style={{ color: leftTextColor }}
        >
          {leftLabel}
        </span>
        <span
          className="text-sm font-medium"
          style={{ color: rightTextColor }}
        >
          {rightLabel}
        </span>
      </div>

      {uploaders.length > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-gray-600 dark:text-gray-400">
          <span>{language === 'zh' ? '上传者：' : 'Uploaders:'}</span>
          {uploaders.map((uploader) => (
            <span key={uploader.id} className="inline-flex items-center gap-1.5">
              <AccountAvatar
                accountId={uploader.id}
                username={uploader.name}
                sizeClassName="w-6 h-6"
              />
              <span className="font-medium text-gray-800 dark:text-gray-200">
                {uploader.name || (language === 'zh' ? '未知用户' : 'Unknown user')}
              </span>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
