'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import { useLanguage } from '@/contexts/LanguageContext';
import { userDetailAPI, RankingItem, UserSearchResultItem } from '@/lib/api';

const SEARCH_DEBOUNCE_MS = 300;

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

export default function RankingsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { t } = useLanguage();

  const [rankings, setRankings] = useState<RankingItem[]>([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mobilePageInput, setMobilePageInput] = useState('');

  // User search (nickname typeahead)
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchResults, setSearchResults] = useState<UserSearchResultItem[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const searchContainerRef = useRef<HTMLDivElement>(null);
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const searchRequestSeq = useRef(0);

  // Build avatar URL with auth token
  const getAvatarUrl = (userId: string) => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    if (!token) return '';
    return `${API_BASE_URL}/user_detail/avatar?userId=${userId}&token=${encodeURIComponent(token)}`;
  };

  // Fetch a page of rankings
  const fetchRankings = useCallback(async (page: number) => {
    try {
      setLoading(true);
      const response = await userDetailAPI.getRankings(page);
      setRankings(response.rankings);
      setTotal(response.total);
      setTotalPages(response.totalPages);
      setCurrentPage(response.page);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch rankings:', err);
      setError('Failed to load rankings');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const pageParam = searchParams.get('page');
    const initialPage = pageParam ? Math.max(1, parseInt(pageParam, 10) || 1) : 1;
    fetchRankings(initialPage);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  // Debounce user search: wait for typing to settle before hitting the API
  useEffect(() => {
    const keyword = searchKeyword.trim();
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
    }

    if (!keyword) {
      setSearchResults([]);
      setSearchLoading(false);
      return;
    }

    setSearchLoading(true);
    searchDebounceRef.current = setTimeout(async () => {
      const requestId = ++searchRequestSeq.current;
      try {
        const response = await userDetailAPI.searchUsers(keyword);
        if (requestId === searchRequestSeq.current) {
          setSearchResults(response.users);
        }
      } catch (err) {
        console.error('Failed to search users:', err);
        if (requestId === searchRequestSeq.current) {
          setSearchResults([]);
        }
      } finally {
        if (requestId === searchRequestSeq.current) {
          setSearchLoading(false);
        }
      }
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      if (searchDebounceRef.current) {
        clearTimeout(searchDebounceRef.current);
      }
    };
  }, [searchKeyword]);

  // Close search dropdown when clicking outside of it
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (searchContainerRef.current && !searchContainerRef.current.contains(event.target as Node)) {
        setSearchOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Navigate to user detail page
  const handleUserClick = (userId: string) => {
    router.push(`/user_stats/${userId}`);
  };

  const handleSearchResultClick = (userId: string) => {
    setSearchOpen(false);
    setSearchKeyword('');
    setSearchResults([]);
    router.push(`/user_stats/${userId}`);
  };

  // Handle page change
  const handlePageChange = (page: number) => {
    if (page >= 1 && page <= totalPages && page !== currentPage) {
      const newParams = new URLSearchParams(searchParams.toString());
      if (page === 1) {
        newParams.delete('page');
      } else {
        newParams.set('page', String(page));
      }
      const paramString = newParams.toString();
      router.push(`/user_stats${paramString ? `?${paramString}` : ''}`);
      window.scrollTo({ top: 0, behavior: 'smooth' });
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

  // Get rank badge color
  const getRankBadgeClass = (rank: number) => {
    if (rank === 1) return 'bg-yellow-400 text-yellow-900';
    if (rank === 2) return 'bg-gray-300 text-gray-700';
    if (rank === 3) return 'bg-amber-600 text-white';
    return 'bg-gray-100 text-gray-600';
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
      <div className="px-4 sm:px-6 py-4 border-t border-gray-100 dark:border-gray-800">
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
              {t('rankings.prev')}
            </button>
            <span className="text-sm text-gray-600 dark:text-gray-400">
              {t('rankings.pageOf', { page: currentPage, totalPages })}
            </span>
            <button
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              className="flex items-center gap-1 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 text-sm dark:text-gray-300"
            >
              {t('rankings.next')}
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </button>
          </div>
          <form onSubmit={handleMobilePageJump} className="flex justify-center items-center gap-2">
            <span className="text-sm text-gray-600 dark:text-gray-400">{t('rankings.jumpTo')}</span>
            <input
              type="number"
              min="1"
              max={totalPages}
              value={mobilePageInput}
              onChange={(e) => setMobilePageInput(e.target.value)}
              placeholder={String(currentPage)}
              className="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            />
            <span className="text-sm text-gray-600 dark:text-gray-400">{t('rankings.page')}</span>
            <button
              type="submit"
              disabled={!mobilePageInput || parseInt(mobilePageInput, 10) < 1 || parseInt(mobilePageInput, 10) > totalPages}
              className="px-3 py-1 text-sm bg-indigo-600 text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-indigo-700 transition"
            >
              {t('rankings.go')}
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
        <PageTitle titleKey="pageTitle.rankings" />
        <Navbar title={t('rankings.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500 dark:text-gray-400">{t('rankings.loading')}</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
        <PageTitle titleKey="pageTitle.rankings" />
        <Navbar title={t('rankings.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">{error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      <PageTitle titleKey="pageTitle.rankings" />
      <Navbar title={t('rankings.title')} showBackButton backHref="/problems" />

      <div className="max-w-4xl mx-auto px-4 py-6">
        {/* User search (nickname typeahead) */}
        <div className="relative mb-4" ref={searchContainerRef}>
          <div className="relative">
            <svg
              className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z" />
            </svg>
            <input
              type="text"
              value={searchKeyword}
              onChange={(e) => {
                setSearchKeyword(e.target.value);
                setSearchOpen(true);
              }}
              onFocus={() => setSearchOpen(true)}
              placeholder={t('rankings.searchPlaceholder')}
              className="w-full pl-9 pr-4 py-2.5 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none shadow-sm"
            />
          </div>

          {searchOpen && searchKeyword.trim() && (
            <div className="absolute z-10 mt-1 w-full bg-white dark:bg-gray-900 rounded-xl shadow-lg border border-gray-100 dark:border-gray-800 max-h-72 overflow-y-auto">
              {searchLoading ? (
                <div className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">{t('rankings.searchLoading')}</div>
              ) : searchResults.length === 0 ? (
                <div className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">{t('rankings.searchNoResults')}</div>
              ) : (
                searchResults.map((user) => (
                  <button
                    key={user.userId}
                    onClick={() => handleSearchResultClick(user.userId)}
                    className="w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-gray-50 dark:hover:bg-gray-800 transition"
                  >
                    <img
                      src={getAvatarUrl(user.userId)}
                      alt="Avatar"
                      className="w-8 h-8 rounded-full object-cover border border-gray-200 dark:border-gray-700 flex-shrink-0"
                      onError={(e) => {
                        (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                      }}
                    />
                    <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">{user.userName}</span>
                  </button>
                ))
              )}
            </div>
          )}
        </div>

        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm overflow-hidden">
          {/* Header */}
          <div className="bg-gradient-to-r from-indigo-500 to-purple-600 px-6 py-4 flex items-center justify-between">
            <h2 className="text-xl font-bold text-white">{t('rankings.title')}</h2>
            {total > 0 && (
              <span className="text-sm text-indigo-100">
                {t('rankings.totalUsers', { count: total })}
              </span>
            )}
          </div>

          {rankings.length === 0 ? (
            <div className="text-center py-12 text-gray-500 dark:text-gray-400">
              {t('rankings.empty')}
            </div>
          ) : (
            <div className="divide-y divide-gray-100 dark:divide-gray-800">
              {/* Table header - hidden on mobile */}
              <div className="hidden sm:grid sm:grid-cols-12 gap-4 px-6 py-3 bg-gray-50 dark:bg-gray-800 text-sm font-medium text-gray-500 dark:text-gray-400">
                <div className="col-span-1">{t('rankings.rank')}</div>
                <div className="col-span-7">{t('rankings.user')}</div>
                <div className="col-span-2 text-right">{t('rankings.likes')}</div>
                <div className="col-span-2 text-right">{t('rankings.comments')}</div>
              </div>

              {/* Rankings list */}
              {rankings.map((item) => (
                <div
                  key={item.userId}
                  className="px-4 sm:px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-800 transition cursor-pointer"
                  onClick={() => handleUserClick(item.userId)}
                >
                  {/* Mobile layout */}
                  <div className="flex sm:hidden items-center gap-3">
                    <span className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold ${getRankBadgeClass(item.rank)}`}>
                      {item.rank}
                    </span>
                    <img
                      src={getAvatarUrl(item.userId)}
                      alt="Avatar"
                      className="w-10 h-10 rounded-full object-cover border border-gray-200 dark:border-gray-700"
                      onError={(e) => {
                        (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                      }}
                    />
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-gray-900 dark:text-gray-100 truncate">{item.userName}</p>
                      <div className="flex gap-3 text-xs text-gray-500 dark:text-gray-400">
                        <span className="flex items-center gap-1">
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5" />
                          </svg>
                          {item.likes.toLocaleString()}
                        </span>
                        <span className="flex items-center gap-1">
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                          </svg>
                          {item.commentCount.toLocaleString()}
                        </span>
                      </div>
                    </div>
                  </div>

                  {/* Desktop layout */}
                  <div className="hidden sm:grid sm:grid-cols-12 gap-4 items-center">
                    <div className="col-span-1">
                      <span className={`inline-flex w-8 h-8 rounded-full items-center justify-center text-sm font-bold ${getRankBadgeClass(item.rank)}`}>
                        {item.rank}
                      </span>
                    </div>
                    <div className="col-span-7 flex items-center gap-3">
                      <img
                        src={getAvatarUrl(item.userId)}
                        alt="Avatar"
                        className="w-10 h-10 rounded-full object-cover border border-gray-200 dark:border-gray-700"
                        onError={(e) => {
                          (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                        }}
                      />
                      <span className="font-medium text-gray-900 dark:text-gray-100">{item.userName}</span>
                    </div>
                    <div className="col-span-2 text-right">
                      <span className="inline-flex items-center gap-1 text-gray-700 dark:text-gray-300">
                        <svg className="w-4 h-4 text-red-400" fill="currentColor" viewBox="0 0 24 24">
                          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
                        </svg>
                        {item.likes.toLocaleString()}
                      </span>
                    </div>
                    <div className="col-span-2 text-right text-gray-500 dark:text-gray-400">
                      {item.commentCount.toLocaleString()}
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
