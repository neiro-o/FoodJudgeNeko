'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useLanguage } from '@/contexts/LanguageContext';
import type { ProblemComment } from '@/lib/api';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

interface CommentListProps {
  comments: ProblemComment[];
  mongoId?: string;
  pageSize?: number;
}

// djb2-style hash over a string → 1-9
function avatarHash(input: string): number {
  let h = 5381;
  for (let i = 0; i < input.length; i++) {
    h = ((h << 5) + h) ^ input.charCodeAt(i);
    h = h >>> 0; // keep uint32
  }
  return (h % 9) + 1;
}

export default function CommentList({ comments, mongoId = '', pageSize = 8 }: CommentListProps) {
  const { language } = useLanguage();
  const router = useRouter();
  const [currentPage, setCurrentPage] = useState(1);

  // Pagination
  const totalPages = Math.ceil(comments.length / pageSize);
  const startIndex = (currentPage - 1) * pageSize;
  const endIndex = startIndex + pageSize;
  const currentComments = comments.slice(startIndex, endIndex);

  // Format timestamp to YYYY/MM/DD H:MM:SS (GMT+8)
  const formatTimestamp = (ts: number): string => {
    const date = new Date(ts * 1000);
    // Force GMT+8
    const utc = date.getTime() + date.getTimezoneOffset() * 60000;
    const gmt8 = new Date(utc + 8 * 3600000);
    const year = gmt8.getFullYear();
    const month = (gmt8.getMonth() + 1).toString().padStart(2, '0');
    const day = gmt8.getDate().toString().padStart(2, '0');
    return `${year}-${month}-${day}`;
  };

  // Get avatar URL from API with token
  const getAvatarUrl = (userid: number): string => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    if (!token) return getDefaultAvatarPath(userid, 0);
    return `${API_BASE_URL}/user_detail/avatar?userId=${userid}&token=${encodeURIComponent(token)}`;
  };

  // Get default avatar path: hash over userid + comment timestamp + mongoId → 1-9
  const getDefaultAvatarPath = (userid: number, timestamp: number): string => {
    const seed = `${userid}:${timestamp}:${mongoId}`;
    return `/avatars/anime_kangaroo_avatar_${avatarHash(seed)}.png`;
  };

  // Handle avatar error - fall back to default avatar
  const handleAvatarError = (e: React.SyntheticEvent<HTMLImageElement>, userid: number, timestamp: number) => {
    const target = e.target as HTMLImageElement;
    const defaultPath = getDefaultAvatarPath(userid, timestamp);
    if (!target.src.includes('/avatars/anime_kangaroo_avatar_')) {
      target.src = defaultPath;
    }
  };

  // Handle avatar click
  const handleAvatarClick = (userid: number) => {
    router.push(`/user_stats/${userid}`);
  };

  const title = language === 'zh' ? '评论区' : 'Comments';

  return (
    <div>
      <h3 className="text-base font-semibold text-gray-900 mb-4">
        {title} ({comments.length})
      </h3>
      
      <div className="space-y-4">
        {currentComments.map((comment) => (
          <div key={comment.id} className="flex gap-3">
            {/* Avatar */}
            <div className="flex-shrink-0">
              <img
                src={getAvatarUrl(comment.userid)}
                alt={comment.name}
                className="w-10 h-10 rounded-full object-cover cursor-pointer hover:opacity-80 transition"
                onClick={() => handleAvatarClick(comment.userid)}
                onError={(e) => handleAvatarError(e, comment.userid, comment.timestamp)}
              />
            </div>

            {/* Content */}
            <div className="flex-1 min-w-0">
              {/* Header: Name + Tag */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm font-medium text-gray-900">{comment.name}</span>
                {comment.choice === 1 ? (
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
                    {language === 'zh' ? '适合展示' : 'Support User'}
                  </span>
                ) : comment.choice === 2 ? (
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-700">
                    {language === 'zh' ? '不适合展示' : 'Support Merchant'}
                  </span>
                ) : null}
              </div>

              {/* Comment Content */}
              <p className="text-sm text-gray-700 mt-1">{comment.content}</p>

              {/* Footer: Time + Likes */}
              <div className="flex items-center gap-4 mt-2 text-xs text-gray-500">
                <span>{formatTimestamp(comment.timestamp)}</span>
                <span className="flex items-center gap-1">
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5" />
                  </svg>
                  {comment.likes}
                </span>
              </div>
            </div>
          </div>
        ))}

        {comments.length === 0 && (
          <p className="text-sm text-gray-500 text-center py-4">
            {language === 'zh' ? '暂无评论' : 'No comments yet'}
          </p>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-4 pt-4 border-t border-gray-100">
          {/* Previous Button */}
          <button
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            disabled={currentPage === 1}
            className="p-1 rounded hover:bg-gray-100 disabled:opacity-30 disabled:cursor-not-allowed transition"
          >
            <svg className="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
          </button>

          {/* Page Numbers */}
          <div className="flex items-center gap-1">
            {Array.from({ length: totalPages }, (_, i) => i + 1).map((page) => (
              <button
                key={page}
                onClick={() => setCurrentPage(page)}
                className={`min-w-[28px] h-7 text-sm rounded transition ${
                  currentPage === page
                    ? 'bg-indigo-600 text-white'
                    : 'text-gray-600 hover:bg-gray-100'
                }`}
              >
                {page}
              </button>
            ))}
          </div>

          {/* Next Button */}
          <button
            onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
            disabled={currentPage === totalPages}
            className="p-1 rounded hover:bg-gray-100 disabled:opacity-30 disabled:cursor-not-allowed transition"
          >
            <svg className="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        </div>
      )}
    </div>
  );
}
