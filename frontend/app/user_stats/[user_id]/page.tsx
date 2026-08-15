'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import Navbar from '@/components/Navbar';
import ImageModal from '@/components/ImageModal';
import { useLanguage } from '@/contexts/LanguageContext';
import { useAuth } from '@/contexts/AuthContext';
import { userDetailAPI, mediaAPI, UserInfoResponse, UserComment, UserCommentsResponse, AIUserSummaryResponse } from '@/lib/api';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

// Check if URL is from an external (proxy-required) domain
function isExternalMediaUrl(url: string): boolean {
  if (!url || url.startsWith('/') || url.startsWith('data:') || url.startsWith('blob:')) {
    return false;
  }
  try {
    const urlObj = new URL(url);
    const host = urlObj.hostname.toLowerCase();
    return (
      host.includes('meituan.com') ||
      host.includes('meituan.net') ||
      host.includes('sankuai.com')
    );
  } catch {
    return false;
  }
}

export default function UserStatsPage() {
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const userId = params.user_id as string;
  const { t } = useLanguage();
  const { user } = useAuth();

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
  const [showMaliciousTooltip, setShowMaliciousTooltip] = useState(false);
  const [showToggleDialog, setShowToggleDialog] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [modalImage, setModalImage] = useState<string | null>(null);
  const [mediaUrlMap, setMediaUrlMap] = useState<Map<string, string>>(new Map());
  const [exportingVanished, setExportingVanished] = useState(false);

  const [aiSummary, setAiSummary] = useState<AIUserSummaryResponse | null>(null);
  const [aiSummaryLoading, setAiSummaryLoading] = useState(true);
  const [aiSummaryGenerating, setAiSummaryGenerating] = useState(false);
  const [aiSummaryError, setAiSummaryError] = useState<string | null>(null);
  const [showFunGuesses, setShowFunGuesses] = useState(false);
  const [showEvidence, setShowEvidence] = useState(false);

  const LIMIT = 10;

  // Resolve a media URL through the proxy for external domains, caching the result
  const getProxiedMediaUrl = (url: string): string => mediaUrlMap.get(url) || url;

  // Format a duration in seconds as M:SS
  const formatDuration = (seconds: number): string => {
    const total = Math.max(0, Math.round(seconds || 0));
    const m = Math.floor(total / 60);
    const s = total % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

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

  // Fetch any cached AI summary (does not trigger generation)
  useEffect(() => {
    const fetchAiSummary = async () => {
      try {
        setAiSummaryLoading(true);
        const summary = await userDetailAPI.getAISummary(userId);
        setAiSummary(summary);
      } catch (err) {
        console.error('Failed to fetch AI summary:', err);
      } finally {
        setAiSummaryLoading(false);
      }
    };

    if (userId) {
      fetchAiSummary();
    }
  }, [userId]);

  // Manually trigger AI summary generation (or fetch a still-fresh cache)
  const handleGenerateAiSummary = async () => {
    setAiSummaryGenerating(true);
    setAiSummaryError(null);
    try {
      const summary = await userDetailAPI.generateAISummary(userId);
      setAiSummary(summary);
    } catch (err: any) {
      setAiSummaryError(err?.message || 'Unknown error');
    } finally {
      setAiSummaryGenerating(false);
    }
  };

  // Preload proxied URLs for images/audios in the current comments
  useEffect(() => {
    const preload = async () => {
      const urls: { url: string; isAudio: boolean }[] = [];
      comments.forEach((comment) => {
        (comment.images || []).forEach((url) => urls.push({ url, isAudio: false }));
        (comment.audios || []).forEach((audio) => urls.push({ url: audio.url, isAudio: true }));
      });

      const uniqueUrls = Array.from(new Map(urls.map((u) => [u.url, u])).values());
      for (const { url, isAudio } of uniqueUrls) {
        if (!isExternalMediaUrl(url) || mediaUrlMap.has(url)) continue;
        try {
          const proxiedUrl = isAudio
            ? await mediaAPI.getAudioUrl(url)
            : await mediaAPI.getImageUrl(url);
          setMediaUrlMap((prev) => new Map(prev).set(url, proxiedUrl));
        } catch (error) {
          console.error('Failed to preload comment media:', url, error);
        }
      }
    };

    preload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [comments]);

  // Close tooltip on Escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && showMaliciousTooltip) {
        setShowMaliciousTooltip(false);
      }
    };
    if (showMaliciousTooltip) {
      document.addEventListener('keydown', handleEscape);
      return () => document.removeEventListener('keydown', handleEscape);
    }
  }, [showMaliciousTooltip]);

  // Format timestamp
  const formatDate = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    });
  };

  // Format a unix-seconds timestamp with date + time (used for AI summary generatedAt)
  const formatDateTime = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const escapeCSVCell = (value: string | number): string => {
    const text = String(value ?? '');
    return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
  };

  const handleExportVanished = async () => {
    setExportingVanished(true);
    try {
      const rows = await userDetailAPI.getVanishedComments(userId);
      if (rows.length === 0) {
        window.alert('你暂时没有被举报的评价。');
        return;
      }

      const columns = [
        'problemId', 'user_review', 'timestamp', 'commentId',
        'content', 'likes', 'createTime', 'url',
      ] as const;
      const csv = [
        columns.join(','),
        ...rows.map((row) => columns.map((column) => escapeCSVCell(row[column])).join(',')),
      ].join('\r\n');
      const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' });
      const downloadUrl = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = downloadUrl;
      link.download = `${userId}.csv`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(downloadUrl);
    } catch (err: any) {
      window.alert(err?.message || '导出被举报评价失败');
    } finally {
      setExportingVanished(false);
    }
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
              className="flex items-center gap-1 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 text-sm dark:text-gray-300"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
              Prev
            </button>
            <span className="text-sm text-gray-600 dark:text-gray-400">
              {currentPage} / {totalPages}
            </span>
            <button
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              className="flex items-center gap-1 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 text-sm dark:text-gray-300"
            >
              Next
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </button>
          </div>
          <form onSubmit={handleMobilePageJump} className="flex justify-center items-center gap-2">
            <span className="text-sm text-gray-600 dark:text-gray-400">跳转到</span>
            <input
              type="number"
              min="1"
              max={totalPages}
              value={mobilePageInput}
              onChange={(e) => setMobilePageInput(e.target.value)}
              placeholder={String(currentPage)}
              className="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            />
            <span className="text-sm text-gray-600 dark:text-gray-400">页</span>
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
            className="px-3 py-1 rounded-lg border border-gray-300 dark:border-gray-700 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 dark:text-gray-300"
          >
            ←
          </button>
          {startPage > 1 && (
            <>
              <button
                onClick={() => handlePageChange(1)}
                className="px-3 py-1 rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 dark:text-gray-300"
              >
                1
              </button>
              {startPage > 2 && <span className="px-2 dark:text-gray-400">...</span>}
            </>
          )}
          {pages.map((page) => (
            <button
              key={page}
              onClick={() => handlePageChange(page)}
              className={`px-3 py-1 rounded-lg border ${
                page === currentPage
                  ? 'bg-indigo-600 text-white border-indigo-600'
                  : 'border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 dark:text-gray-300'
              }`}
            >
              {page}
            </button>
          ))}
          {endPage < totalPages && (
            <>
              {endPage < totalPages - 1 && <span className="px-2 dark:text-gray-400">...</span>}
              <button
                onClick={() => handlePageChange(totalPages)}
                className="px-3 py-1 rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 dark:text-gray-300"
              >
                {totalPages}
              </button>
            </>
          )}
          <button
            onClick={() => handlePageChange(currentPage + 1)}
            disabled={currentPage === totalPages}
            className="px-3 py-1 rounded-lg border border-gray-300 dark:border-gray-700 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 dark:text-gray-300"
          >
            →
          </button>
        </div>
      </div>
    );
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
        <Navbar title={t('userStats.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500 dark:text-gray-400">{t('userStats.loading')}</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
        <Navbar title={t('userStats.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">{error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      <Navbar title={t('userStats.title')} showBackButton backHref="/problems" />

      <div className="max-w-4xl mx-auto px-4 py-6">
        {/* User Profile Section */}
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm p-6 mb-6">
          <div className="flex items-start gap-4">
            {/* Avatar */}
            <div className="flex-shrink-0">
              {avatarUrl ? (
                <img
                  src={avatarUrl}
                  alt="Avatar"
                  className={`w-16 h-16 rounded-full object-cover border-2 border-gray-200 dark:border-gray-700 ${
                    user?.is_admin ? 'cursor-pointer hover:opacity-80 transition-opacity' : ''
                  }`}
                  onDoubleClick={(e) => {
                    if (user?.is_admin) {
                      e.stopPropagation();
                      setShowToggleDialog(true);
                    }
                  }}
                  onError={(e) => {
                    (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                  }}
                  title={user?.is_admin ? 'Double-click to toggle malicious status' : undefined}
                />
              ) : (
                <div className="w-16 h-16 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center">
                  <svg className="w-8 h-8 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/>
                  </svg>
                </div>
              )}
            </div>

            {/* User Info */}
            <div className="flex-1">
              <div className="flex flex-wrap items-center gap-2 mb-2">
                <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
                  {userInfo?.userName || 'Unknown User'}
                </h2>
                <button
                  type="button"
                  onClick={handleExportVanished}
                  disabled={exportingVanished}
                  className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {exportingVanished ? '导出中...' : '导出被举报评价'}
                </button>
                {userInfo?.malicious && (
                  <div className="relative">
                    <span 
                      className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 dark:bg-red-900/40 text-red-800 dark:text-red-300 border border-red-300 dark:border-red-700 cursor-pointer hover:bg-red-200 dark:hover:bg-red-900/60 transition-colors"
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowMaliciousTooltip(!showMaliciousTooltip);
                      }}
                    >
                      {t('userStats.maliciousAccount')}
                    </span>
                    {showMaliciousTooltip && (
                      <>
                        <div 
                          className="fixed inset-0 z-40"
                          onClick={() => setShowMaliciousTooltip(false)}
                        />
                        <div className="absolute left-0 top-full mt-2 z-50 w-80 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg">
                          <p className="whitespace-normal leading-relaxed">
                            {t('userStats.maliciousAccountTooltip')}
                          </p>
                          <div className="absolute -top-1 left-4 w-2 h-2 bg-gray-900 transform rotate-45"></div>
                        </div>
                      </>
                    )}
                  </div>
                )}
              </div>
              <div className="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                <p>{t('userStats.totalLikes', { count: userInfo?.likes || 0 })}</p>
                <p>{t('userStats.totalReplies', { count: userInfo?.replies || 0 })}</p>
              </div>
            </div>
          </div>
        </div>

        {/* Toggle Malicious Dialog */}
        {showToggleDialog && (
          <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
                {userInfo?.malicious ? 'Untag Malicious User' : 'Tag Malicious User'}
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-6">
                {userInfo?.malicious
                  ? `Are you sure you want to remove the malicious tag from ${userInfo?.userName || 'this user'}?`
                  : `Are you sure you want to tag ${userInfo?.userName || 'this user'} as malicious?`}
              </p>
              <div className="flex justify-end gap-3">
                <button
                  onClick={() => setShowToggleDialog(false)}
                  disabled={toggling}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  onClick={async () => {
                    setToggling(true);
                    try {
                      await userDetailAPI.toggleMalicious(userId);
                      // Refresh user info
                      const info = await userDetailAPI.getUserInfo(userId);
                      setUserInfo(info);
                      setShowToggleDialog(false);
                    } catch (err: any) {
                      console.error('Failed to toggle malicious status:', err);
                      alert(err.message || 'Failed to toggle malicious status');
                    } finally {
                      setToggling(false);
                    }
                  }}
                  disabled={toggling}
                  className={`px-4 py-2 text-sm font-medium text-white rounded-lg transition disabled:opacity-50 ${
                    userInfo?.malicious
                      ? 'bg-green-600 hover:bg-green-700'
                      : 'bg-red-600 hover:bg-red-700'
                  }`}
                >
                  {toggling ? 'Processing...' : userInfo?.malicious ? 'Untag' : 'Tag'}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* AI Comment Profile Section */}
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm p-6 mb-6">
          <div className="flex items-center justify-between gap-3 mb-4 flex-wrap">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {t('aiSummary.title')}
            </h3>
            {!aiSummaryLoading && aiSummary && aiSummary.status === 'ready' && (
              <button
                onClick={handleGenerateAiSummary}
                disabled={aiSummaryGenerating || !aiSummary.stale}
                className={`px-3 py-1.5 text-sm font-medium rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed ${
                  aiSummary.stale
                    ? 'bg-indigo-600 text-white hover:bg-indigo-700'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-500'
                }`}
              >
                {aiSummaryGenerating
                  ? t('aiSummary.generating')
                  : aiSummary.stale
                    ? t('aiSummary.refresh')
                    : t('aiSummary.refreshLocked')}
              </button>
            )}
          </div>

          {aiSummaryLoading ? (
            <div className="text-center py-6 text-gray-500 dark:text-gray-400 text-sm">
              {t('aiSummary.loading')}
            </div>
          ) : aiSummary && aiSummary.status === 'ready' && aiSummary.result ? (
            <div className="space-y-4">
              {aiSummary.generatedAt && (
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  {t('aiSummary.lastGenerated', {
                    date: formatDateTime(aiSummary.generatedAt),
                    provider: aiSummary.provider || '',
                  })}
                </p>
              )}

              {/* Comment quality stars (1-5); omit when absent on older summaries */}
              {typeof aiSummary.result.commentQualityStars === 'number' &&
                aiSummary.result.commentQualityStars >= 1 &&
                aiSummary.result.commentQualityStars <= 5 && (
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {t('aiSummary.qualityStars')}
                  </span>
                  <span
                    className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-sm font-medium bg-amber-50 dark:bg-amber-950/40 text-amber-800 dark:text-amber-200 border border-amber-200 dark:border-amber-800"
                    title={`${aiSummary.result.commentQualityStars}/5`}
                  >
                    <span className="tracking-tight" aria-hidden="true">
                      {'★'.repeat(aiSummary.result.commentQualityStars)}
                      {'☆'.repeat(5 - aiSummary.result.commentQualityStars)}
                    </span>
                    <span>
                      {t(`aiSummary.qualityStars.${aiSummary.result.commentQualityStars}`)}
                    </span>
                  </span>
                </div>
              )}

              {/* Roast */}
              <div className="p-4 rounded-lg bg-gradient-to-br from-indigo-50 to-purple-50 dark:from-indigo-950/40 dark:to-purple-950/40 border border-indigo-100 dark:border-indigo-900">
                <p className="text-gray-800 dark:text-gray-100 leading-relaxed">
                  {aiSummary.result.roast}
                </p>
              </div>

              {/* Profile summary */}
              <p className="text-sm text-gray-700 dark:text-gray-300">
                {aiSummary.result.profile.summary}
              </p>

              {/* Feature groups */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                {([
                  ['aiSummary.expressionStyle', aiSummary.result.profile.expressionStyle],
                  ['aiSummary.opinionTendency', aiSummary.result.profile.opinionTendency],
                  ['aiSummary.interactionPattern', aiSummary.result.profile.interactionPattern],
                ] as const).map(([labelKey, items]) =>
                  items && items.length > 0 ? (
                    <div key={labelKey} className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                      <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">
                        {t(labelKey)}
                      </p>
                      <div className="flex flex-wrap gap-1.5">
                        {items.map((item, idx) => (
                          <span
                            key={idx}
                            className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-indigo-100 dark:bg-indigo-900/40 text-indigo-800 dark:text-indigo-300"
                          >
                            {item}
                          </span>
                        ))}
                      </div>
                    </div>
                  ) : null
                )}
              </div>

              {/* Limitations */}
              {aiSummary.result.limitations && aiSummary.result.limitations.length > 0 && (
                <div>
                  <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                    {t('aiSummary.limitations')}
                  </p>
                  <ul className="list-disc list-inside text-xs text-gray-500 dark:text-gray-400 space-y-0.5">
                    {aiSummary.result.limitations.map((limitation, idx) => (
                      <li key={idx}>{limitation}</li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Fun guesses (entertainment only, collapsed by default) */}
              <div className="border-t border-gray-100 dark:border-gray-800 pt-3">
                <button
                  onClick={() => setShowFunGuesses(!showFunGuesses)}
                  className="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 flex items-center gap-1"
                >
                  <span>{showFunGuesses ? '▾' : '▸'}</span>
                  {t('aiSummary.funGuessToggle')}
                </button>
                {showFunGuesses && (
                  <div className="mt-2 grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {([
                      ['aiSummary.gender', aiSummary.result.profile.genderGuess],
                      ['aiSummary.mbti', aiSummary.result.profile.mbtiGuess],
                    ] as const).map(([labelKey, guess]) => (
                      <div key={labelKey} className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                        <p className="text-xs font-medium text-gray-500 dark:text-gray-400">
                          {t(labelKey)}
                        </p>
                        <p className="text-sm text-gray-800 dark:text-gray-200 mt-0.5">
                          {guess.value}
                        </p>
                        <p className="text-[11px] text-gray-400 dark:text-gray-500 mt-0.5">
                          {guess.disclaimer}
                        </p>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Supporting evidence (collapsed by default) */}
              {aiSummary.result.evidence && aiSummary.result.evidence.length > 0 && (
                <div className="border-t border-gray-100 dark:border-gray-800 pt-3">
                  <button
                    onClick={() => setShowEvidence(!showEvidence)}
                    className="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 flex items-center gap-1"
                  >
                    <span>{showEvidence ? '▾' : '▸'}</span>
                    {t('aiSummary.evidenceToggle')}
                  </button>
                  {showEvidence && (
                    <ul className="mt-2 space-y-2">
                      {aiSummary.result.evidence.map((item, idx) => (
                        <li key={idx} className="text-xs text-gray-600 dark:text-gray-300 p-2 rounded bg-gray-50 dark:bg-gray-800">
                          <p className="font-medium text-gray-700 dark:text-gray-200">{item.claim}</p>
                          <p className="mt-0.5">{item.reason}</p>
                          {item.evidenceIds && item.evidenceIds.length > 0 && (
                            <p className="mt-1 text-gray-400 dark:text-gray-500">
                              {item.evidenceIds.map((id) => t('aiSummary.sample', { id })).join('、')}
                            </p>
                          )}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}

              {aiSummaryError && (
                <p className="text-xs text-red-500">{t('aiSummary.error', { message: aiSummaryError })}</p>
              )}
            </div>
          ) : (
            <div className="text-center py-6">
              {aiSummary && aiSummary.status === 'failed' && (
                <p className="text-sm text-red-500 mb-3">
                  {t('aiSummary.error', { message: aiSummary.lastError || 'Unknown error' })}
                </p>
              )}
              {aiSummaryError && (
                <p className="text-sm text-red-500 mb-3">{t('aiSummary.error', { message: aiSummaryError })}</p>
              )}
              {!aiSummaryError && (!aiSummary || aiSummary.status === 'none') && (
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-3">
                  {t('aiSummary.emptyState')}
                </p>
              )}
              <button
                onClick={handleGenerateAiSummary}
                disabled={aiSummaryGenerating}
                className="px-4 py-2 text-sm font-medium bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {aiSummaryGenerating
                  ? t('aiSummary.generating')
                  : aiSummary && aiSummary.status === 'failed'
                  ? t('aiSummary.retry')
                  : t('aiSummary.generate')}
              </button>
            </div>
          )}
        </div>

        {/* Comments Section */}
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm p-6">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
            {t('userStats.comments', { count: totalComments })}
          </h3>

          {commentsLoading ? (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400">
              {t('userStats.loading')}
            </div>
          ) : comments.length === 0 ? (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400">
              {t('userStats.noComments')}
            </div>
          ) : (
            <div className="space-y-4">
              {comments.map((comment) => (
                <div
                  key={comment.id}
                  className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-800 transition"
                >
                  <div className="flex items-start gap-3">
                    <div 
                      className="flex-shrink-0 cursor-pointer"
                      onClick={() => router.push(`/problems/${comment.problemId}`)}
                      title="View Problem"
                    >
                      {avatarUrl ? (
                        <img
                          src={avatarUrl}
                          alt="Avatar"
                          className="w-10 h-10 rounded-full object-cover border border-gray-200 dark:border-gray-700 hover:ring-2 hover:ring-indigo-400 transition"
                          onError={(e) => {
                            (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                          }}
                        />
                      ) : (
                        <div className="w-10 h-10 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center hover:ring-2 hover:ring-indigo-400 transition">
                          <svg className="w-5 h-5 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/>
                          </svg>
                        </div>
                      )}
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <span className="font-medium text-gray-900 dark:text-gray-100">
                          {comment.userName}
                        </span>
                        {comment.choice === 1 && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300">
                            {t('userStats.supportUser')}
                          </span>
                        )}
                        {comment.choice === 2 && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
                            {t('userStats.supportMerchant')}
                          </span>
                        )}
                      </div>

                      <p className="text-gray-700 dark:text-gray-300 text-sm mb-2 break-words">
                        {comment.content}
                      </p>

                      {/* Audio player bar(s) */}
                      {comment.audios && comment.audios.length > 0 && (
                        <div className="space-y-2 max-w-md mb-2">
                          {comment.audios.map((audio, idx) => (
                            <div key={idx}>
                              <div className="flex items-center gap-2">
                                <audio
                                  controls
                                  preload="none"
                                  src={getProxiedMediaUrl(audio.url)}
                                  className="flex-1 h-8"
                                />
                                <span className="text-xs text-gray-500 dark:text-gray-400 flex-shrink-0">
                                  {formatDuration(audio.duration)}
                                </span>
                              </div>
                              {audio.audioText && (
                                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                  {audio.audioText}
                                </p>
                              )}
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Image thumbnails */}
                      {comment.images && comment.images.length > 0 && (
                        <div className="flex flex-wrap gap-2 mb-2">
                          {comment.images.map((imageUrl, idx) => (
                            <img
                              key={idx}
                              src={getProxiedMediaUrl(imageUrl)}
                              alt={`${comment.userName} image ${idx + 1}`}
                              className="w-16 h-16 object-cover rounded cursor-pointer hover:opacity-80 transition"
                              onClick={(e) => {
                                e.stopPropagation();
                                setModalImage(imageUrl);
                              }}
                            />
                          ))}
                        </div>
                      )}

                      <div className="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
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

      {/* Image Modal */}
      <ImageModal
        imageUrl={modalImage ? getProxiedMediaUrl(modalImage) : ''}
        isOpen={!!modalImage}
        onClose={() => setModalImage(null)}
      />
    </div>
  );
}
