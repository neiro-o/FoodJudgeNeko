'use client';

import { useEffect, useState } from 'react';
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
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setFailed(false);
  }, [accountId]);

  const sharedClassName = `${sizeClassName} flex-shrink-0 rounded-full ${className}`;
  const initial = Array.from(username.trim())[0]?.toUpperCase() || '?';

  if (!accountId || failed) {
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
    <img
      src={accountAPI.getAvatarUrl(accountId)}
      alt={username ? `${username} avatar` : 'Account avatar'}
      className={`${sharedClassName} object-cover bg-gray-100 dark:bg-gray-800`}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}
