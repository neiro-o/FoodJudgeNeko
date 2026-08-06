'use client';

import { useLanguage } from '@/contexts/LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';
import AccountAvatar from './AccountAvatar';

interface RatioBarProps {
  ratio1: number;
  ratio2: number;
  answer: number; // 1 = support user, 2 = support merchant
  uploaderId?: string;
  uploaderName?: string;
}

export default function RatioBar({ ratio1, ratio2, answer, uploaderId, uploaderName }: RatioBarProps) {
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

      {uploaderId && (
        <div className="mt-3 flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <span>{language === 'zh' ? '最后上传人：' : 'Last uploader:'}</span>
          <AccountAvatar
            accountId={uploaderId}
            username={uploaderName}
            sizeClassName="w-6 h-6"
          />
          <span className="font-medium text-gray-800 dark:text-gray-200">
            {uploaderName || (language === 'zh' ? '未知用户' : 'Unknown user')}
          </span>
        </div>
      )}
    </div>
  );
}
