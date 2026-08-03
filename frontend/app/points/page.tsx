'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { pointsAPI, LeaderboardEntry } from '@/lib/api';

const PAGE_LIMIT = 50;

export default function PointsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { isAuthenticated, loading: authLoading } = useAuth();
  const { t } = useLanguage();

  const [rankings, setRankings] = useState<LeaderboardEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(0);
  const [year, setYear] = useState<number | null>(null);
  const [weekId, setWeekId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mobilePageInput, setMobilePageInput] = useState('');

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, authLoading, router]);

  const fetchLeaderboard = useCallback(async (page: number) => {
    try {
      setLoading(true);
      const response = await pointsAPI.getLeaderboard(page, PAGE_LIMIT);
      setRankings(response.rankings);
      setTotal(response.total);
      setTotalPages(response.totalPages);
      setCurrentPage(response.page);
      setYear(response.year);
      setWeekId(response.weekId);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch leaderboard:', err);
      setError('Failed to load leaderboard');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!isAuthenticated) return;
    const pageParam = searchParams.get('page');
    const initialPage = pageParam ? Math.max(1, parseInt(pageParam, 10) || 1) : 1;
    fetchLeaderboard(initialPage);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams, isAuthenticated]);

  const handlePageChange = (page: number) => {
    if (page >= 1 && page <= totalPages && page !== currentPage) {
      const newParams = new URLSearchParams(searchParams.toString());
      if (page === 1) {
        newParams.delete('page');
      } else {
        newParams.set('page', String(page));
      }
      const paramString = newParams.toString();
      router.push(`/points${paramString ? `?${paramString}` : ''}`);
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  };

  const handleMobilePageJump = (e: React.FormEvent) => {
    e.preventDefault();
    const page = parseInt(mobilePageInput, 10);
    if (!isNaN(page) && page >= 1 && page <= totalPages) {
      handlePageChange(page);
      setMobilePageInput('');
    }
  };

  const getRankBadgeClass = (rank: number) => {
    if (rank === 1) return 'bg-yellow-400 text-yellow-900';
    if (rank === 2) return 'bg-gray-300 text-gray-700';
    if (rank === 3) return 'bg-amber-600 text-white';
    return 'bg-gray-100 text-gray-600';
  };

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

  if (authLoading || (loading && rankings.length === 0)) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
        <PageTitle titleKey="pageTitle.points" />
        <Navbar title={t('points.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500 dark:text-gray-400">{t('points.loading')}</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
        <PageTitle titleKey="pageTitle.points" />
        <Navbar title={t('points.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">{error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      <PageTitle titleKey="pageTitle.points" />
      <Navbar title={t('points.title')} showBackButton backHref="/problems" />

      <div className="max-w-4xl mx-auto px-4 py-6">
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm overflow-hidden">
          {/* Header */}
          <div className="bg-gradient-to-r from-indigo-500 to-purple-600 px-6 py-4 flex items-center justify-between flex-wrap gap-2">
            <div>
              <h2 className="text-xl font-bold text-white">{t('points.title')}</h2>
              {year !== null && weekId !== null && (
                <p className="text-sm text-indigo-100">{t('points.weekLabel', { year, weekId })}</p>
              )}
            </div>
            {total > 0 && (
              <span className="text-sm text-indigo-100">
                {t('points.totalUsers', { count: total })}
              </span>
            )}
          </div>

          {rankings.length === 0 ? (
            <div className="text-center py-12 text-gray-500 dark:text-gray-400">
              {t('points.empty')}
            </div>
          ) : (
            <div className="divide-y divide-gray-100 dark:divide-gray-800">
              {/* Table header - hidden on mobile */}
              <div className="hidden sm:grid sm:grid-cols-12 gap-4 px-6 py-3 bg-gray-50 dark:bg-gray-800 text-sm font-medium text-gray-500 dark:text-gray-400">
                <div className="col-span-1">{t('points.rank')}</div>
                <div className="col-span-6">{t('points.user')}</div>
                <div className="col-span-2 text-right">{t('points.weeklyScore')}</div>
                <div className="col-span-3 text-right">{t('points.points')}</div>
              </div>

              {/* Leaderboard list */}
              {rankings.map((item, idx) => {
                const rank = (currentPage - 1) * PAGE_LIMIT + idx + 1;
                return (
                  <div
                    key={item.id}
                    className="px-4 sm:px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-800 transition"
                  >
                    {/* Mobile layout */}
                    <div className="flex sm:hidden items-center gap-3">
                      <span className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold ${getRankBadgeClass(rank)}`}>
                        {rank}
                      </span>
                      <div className="flex-1 min-w-0">
                        <p className="font-medium text-gray-900 dark:text-gray-100 truncate">{item.username}</p>
                        <div className="flex gap-3 text-xs text-gray-500 dark:text-gray-400">
                          <span>{t('points.points')}: {item.points.toLocaleString()}</span>
                          <span>
                            {t('points.weeklyScore')}: {item.score !== null ? item.score.toLocaleString() : t('points.noScore')}
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Desktop layout */}
                    <div className="hidden sm:grid sm:grid-cols-12 gap-4 items-center">
                      <div className="col-span-1">
                        <span className={`inline-flex w-8 h-8 rounded-full items-center justify-center text-sm font-bold ${getRankBadgeClass(rank)}`}>
                          {rank}
                        </span>
                      </div>
                      <div className="col-span-6">
                        <span className="font-medium text-gray-900 dark:text-gray-100">{item.username}</span>
                      </div>
                      <div className="col-span-2 text-right text-gray-700 dark:text-gray-300">
                        {item.score !== null ? item.score.toLocaleString() : (
                          <span className="text-gray-400 dark:text-gray-500">{t('points.noScore')}</span>
                        )}
                      </div>
                      <div className="col-span-3 text-right text-gray-500 dark:text-gray-400">
                        {item.points.toLocaleString()}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {/* Pagination */}
          {renderPagination()}
        </div>
      </div>
    </div>
  );
}
