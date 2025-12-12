'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import Navbar from '@/components/Navbar';
import { useLanguage } from '@/contexts/LanguageContext';
import { userDetailAPI, UserInfoResponse, UserComment, UserCommentsResponse } from '@/lib/api';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

export default function UserStatsPage() {
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const userId = params.user_id as string;
  const { t } = useLanguage();

  const [userInfo, setUserInfo] = useState<UserInfoResponse | null>(null);
  const [comments, setComments] = useState<UserComment[]>([]);
  const [totalComments, setTotalComments] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(0);
  const [loading, setLoading] = useState(true);
  const [commentsLoading, setCommentsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [avatarUrl, setAvatarUrl] = useState<string | null>(null);
  const [mobilePageInput, setMobilePageInput] = useState('');

  const LIMIT = 10;

  // Build avatar URL with auth token
  const getAvatarUrl = useCallback(() => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    if (!token) return null;
    return `${API_BASE_URL}/user_detail/avatar?userId=${userId}&token=${encodeURIComponent(token)}`;
  }, [userId]);

  // Fetch user info
  useEffect(() => {
    const fetchUserInfo = async () => {
      try {
        setLoading(true);
        const info = await userDetailAPI.getUserInfo(userId);
        setUserInfo(info);
        setAvatarUrl(getAvatarUrl());
        setError(null);
      } catch (err) {
        setError(t('userStats.notFound'));
      } finally {
        setLoading(false);
      }
    };

    if (userId) {
      fetchUserInfo();
    }
  }, [userId, t, getAvatarUrl]);

  // Update page title with user name
  useEffect(() => {
    if (userInfo?.userName) {
      document.title = userInfo.userName;
    }
  }, [userInfo?.userName]);

  // Fetch comments
  const fetchComments = useCallback(async (page: number) => {
    try {
      setCommentsLoading(true);
      const response = await userDetailAPI.getComments(userId, page, LIMIT);
      setComments(response.comments);
      setTotalComments(response.total);
      setTotalPages(response.totalPages);
      setCurrentPage(page);
    } catch (err) {
      console.error('Failed to fetch comments:', err);
    } finally {
      setCommentsLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    if (userId && !loading) {
      // Read page number from URL params
      const pageParam = searchParams.get('pn');
      const initialPage = pageParam ? Math.max(1, parseInt(pageParam, 10) || 1) : 1;
      fetchComments(initialPage);
    }
  }, [userId, loading, fetchComments, searchParams]);

  // Format timestamp
  const formatDate = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    });
  };

  // Handle page change
  const handlePageChange = (page: number) => {
    if (page >= 1 && page <= totalPages) {
      fetchComments(page);
      window.scrollTo({ top: 0, behavior: 'smooth' });
      
      // Update URL with page number
      const newParams = new URLSearchParams(searchParams.toString());
      if (page === 1) {
        newParams.delete('pn');
      } else {
        newParams.set('pn', String(page));
      }
      const paramString = newParams.toString();
      router.push(`/user_stats/${userId}${paramString ? `?${paramString}` : ''}`);
    }
  };

  // Handle mobile page jump
  const handleMobilePageJump = (e: React.FormEvent) => {
    e.preventDefault();
    const page = parseInt(mobilePageInput, 10);
    if (!isNaN(page) && page >= 1 && page <= totalPages) {
      handlePageChange(page);
      setMobilePageInput('');
    }
  };

  // Render pagination
  const renderPagination = () => {
    if (totalPages <= 1) return null;

    const pages = [];
    const maxVisiblePages = 5;
    let startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
    let endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);

    if (endPage - startPage + 1 < maxVisiblePages) {
      startPage = Math.max(1, endPage - maxVisiblePages + 1);
    }

    for (let i = startPage; i <= endPage; i++) {
      pages.push(i);
    }

    return (
      <div className="mt-6">
        {/* Mobile pagination - prev/next with page jump */}
        <div className="flex sm:hidden flex-col gap-3">
          <div className="flex justify-between items-center">
            <button
              onClick={() => handlePageChange(currentPage - 1)}
              disabled={currentPage === 1}
              className="flex items-center gap-1 px-4 py-2 rounded-lg border border-gray-300 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 text-sm"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
              Prev
            </button>
            <span className="text-sm text-gray-600">
              {currentPage} / {totalPages}
            </span>
            <button
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              className="flex items-center gap-1 px-4 py-2 rounded-lg border border-gray-300 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 text-sm"
            >
              Next
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </button>
          </div>
          {/* Page jump input */}
          <form onSubmit={handleMobilePageJump} className="flex justify-center items-center gap-2">
            <span className="text-sm text-gray-600">跳转到</span>
            <input
              type="number"
              min="1"
              max={totalPages}
              value={mobilePageInput}
              onChange={(e) => setMobilePageInput(e.target.value)}
              placeholder={String(currentPage)}
              className="w-16 px-2 py-1 text-sm text-center border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
            />
            <span className="text-sm text-gray-600">页</span>
            <button
              type="submit"
              disabled={!mobilePageInput || parseInt(mobilePageInput, 10) < 1 || parseInt(mobilePageInput, 10) > totalPages}
              className="px-3 py-1 text-sm bg-indigo-600 text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-indigo-700 transition"
            >
              Go
            </button>
          </form>
        </div>

        {/* Desktop pagination - full page numbers */}
        <div className="hidden sm:flex justify-center items-center gap-2">
          <button
            onClick={() => handlePageChange(currentPage - 1)}
            disabled={currentPage === 1}
            className="px-3 py-1 rounded-lg border border-gray-300 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100"
          >
            ←
          </button>
          {startPage > 1 && (
            <>
              <button
                onClick={() => handlePageChange(1)}
                className="px-3 py-1 rounded-lg border border-gray-300 hover:bg-gray-100"
              >
                1
              </button>
              {startPage > 2 && <span className="px-2">...</span>}
            </>
          )}
          {pages.map((page) => (
            <button
              key={page}
              onClick={() => handlePageChange(page)}
              className={`px-3 py-1 rounded-lg border ${
                page === currentPage
                  ? 'bg-indigo-600 text-white border-indigo-600'
                  : 'border-gray-300 hover:bg-gray-100'
              }`}
            >
              {page}
            </button>
          ))}
          {endPage < totalPages && (
            <>
              {endPage < totalPages - 1 && <span className="px-2">...</span>}
              <button
                onClick={() => handlePageChange(totalPages)}
                className="px-3 py-1 rounded-lg border border-gray-300 hover:bg-gray-100"
              >
                {totalPages}
              </button>
            </>
          )}
          <button
            onClick={() => handlePageChange(currentPage + 1)}
            disabled={currentPage === totalPages}
            className="px-3 py-1 rounded-lg border border-gray-300 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100"
          >
            →
          </button>
        </div>
      </div>
    );
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50">
        <Navbar title={t('userStats.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500">{t('userStats.loading')}</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50">
        <Navbar title={t('userStats.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">{error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Navbar title={t('userStats.title')} showBackButton backHref="/problems" />

      <div className="max-w-4xl mx-auto px-4 py-6">
        {/* User Profile Section */}
        <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
          <div className="flex items-start gap-4">
            {/* Avatar */}
            <div className="flex-shrink-0">
              {avatarUrl ? (
                <img
                  src={avatarUrl}
                  alt="Avatar"
                  className="w-16 h-16 rounded-full object-cover border-2 border-gray-200"
                  onError={(e) => {
                    (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                  }}
                />
              ) : (
                <div className="w-16 h-16 rounded-full bg-gray-200 flex items-center justify-center">
                  <svg className="w-8 h-8 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/>
                  </svg>
                </div>
              )}
            </div>

            {/* User Info */}
            <div className="flex-1">
              <h2 className="text-xl font-semibold text-gray-900 mb-2">
                {userInfo?.userName || 'Unknown User'}
              </h2>
              <div className="text-sm text-gray-600 space-y-1">
                <p>{t('userStats.totalLikes', { count: userInfo?.likes || 0 })}</p>
                <p>{t('userStats.totalReplies', { count: userInfo?.replies || 0 })}</p>
              </div>
            </div>
          </div>
        </div>

        {/* Comments Section */}
        <div className="bg-white rounded-xl shadow-sm p-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">
            {t('userStats.comments', { count: totalComments })}
          </h3>

          {commentsLoading ? (
            <div className="text-center py-8 text-gray-500">
              {t('userStats.loading')}
            </div>
          ) : comments.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              {t('userStats.noComments')}
            </div>
          ) : (
            <div className="space-y-4">
              {comments.map((comment) => (
                <div
                  key={comment.id}
                  className="border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition"
                >
                  <div className="flex items-start gap-3">
                    {/* Comment Avatar - reuse the same avatar, click to go to problem */}
                    <div 
                      className="flex-shrink-0 cursor-pointer"
                      onClick={() => router.push(`/problems/${comment.problemId}`)}
                      title="View Problem"
                    >
                      {avatarUrl ? (
                        <img
                          src={avatarUrl}
                          alt="Avatar"
                          className="w-10 h-10 rounded-full object-cover border border-gray-200 hover:ring-2 hover:ring-indigo-400 transition"
                          onError={(e) => {
                            (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                          }}
                        />
                      ) : (
                        <div className="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center hover:ring-2 hover:ring-indigo-400 transition">
                          <svg className="w-5 h-5 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/>
                          </svg>
                        </div>
                      )}
                    </div>

                    {/* Comment Content */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <span className="font-medium text-gray-900">
                          {comment.userName}
                        </span>
                        {/* Choice Tag */}
                        {comment.choice === 1 && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800">
                            {t('userStats.supportUser')}
                          </span>
                        )}
                        {comment.choice === 2 && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
                            {t('userStats.supportMerchant')}
                          </span>
                        )}
                      </div>

                      {/* Comment Text */}
                      <p className="text-gray-700 text-sm mb-2 break-words">
                        {comment.content}
                      </p>

                      {/* Comment Footer */}
                      <div className="flex items-center gap-4 text-xs text-gray-500">
                        <span>{formatDate(comment.createTime)}</span>
                        <span className="flex items-center gap-1">
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5" />
                          </svg>
                          {comment.approveCount}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Pagination */}
          {renderPagination()}
        </div>
      </div>
    </div>
  );
}
