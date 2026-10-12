'use client';

import { PreferenceImg } from '@/components/PreferenceImage';

import { useEffect, useState } from 'react';
import { useDisplayMode } from '@/contexts/DisplayModeContext';
import { accountAPI } from '@/lib/api';

interface AccountAvatarProps {
  accountId: string;
  username?: string;
  sizeClassName?: string;
  className?: string;
}

export default function AccountAvatar({
  accountId,
  username = '',
  sizeClassName = 'w-10 h-10',
  className = '',
}: AccountAvatarProps) {
  const { isEnergySaving } = useDisplayMode();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setFailed(false);
  }, [accountId]);

  const sharedClassName = `${sizeClassName} flex-shrink-0 rounded-full ${className}`;
  const initial = Array.from(username.trim())[0]?.toUpperCase() || '?';

  if (!accountId || failed || isEnergySaving) {
    return (
      <span
        className={`${sharedClassName} inline-flex items-center justify-center bg-indigo-100 dark:bg-indigo-900/50 text-indigo-700 dark:text-indigo-200 font-semibold`}
        aria-label={username ? `${username} avatar` : 'Account avatar'}
      >
        {initial}
      </span>
    );
  }

  return (
    <PreferenceImg
      src={accountAPI.getAvatarUrl(accountId)}
      alt={username ? `${username} avatar` : 'Account avatar'}
      className={`${sharedClassName} object-cover bg-gray-100 dark:bg-gray-800`}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}
