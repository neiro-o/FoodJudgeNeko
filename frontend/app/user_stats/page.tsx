'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import { useLanguage } from '@/contexts/LanguageContext';
import { userDetailAPI, RankingItem } from '@/lib/api';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

export default function RankingsPage() {
  const router = useRouter();
  const { t } = useLanguage();

  const [rankings, setRankings] = useState<RankingItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Build avatar URL with auth token
  const getAvatarUrl = (userId: string) => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    if (!token) return '';
    return `${API_BASE_URL}/user_detail/avatar?userId=${userId}&token=${encodeURIComponent(token)}`;
  };

  // Fetch rankings
  useEffect(() => {
    const fetchRankings = async () => {
      try {
        setLoading(true);
        const response = await userDetailAPI.getRankings();
        setRankings(response.rankings);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch rankings:', err);
        setError('Failed to load rankings');
      } finally {
        setLoading(false);
      }
    };

    fetchRankings();
  }, []);

  // Navigate to user detail page
  const handleUserClick = (userId: string) => {
    router.push(`/user_stats/${userId}`);
  };

  // Get rank badge color
  const getRankBadgeClass = (rank: number) => {
    if (rank === 1) return 'bg-yellow-400 text-yellow-900';
    if (rank === 2) return 'bg-gray-300 text-gray-700';
    if (rank === 3) return 'bg-amber-600 text-white';
    return 'bg-gray-100 text-gray-600';
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50">
        <PageTitle titleKey="pageTitle.rankings" />
        <Navbar title={t('rankings.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500">{t('rankings.loading')}</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50">
        <PageTitle titleKey="pageTitle.rankings" />
        <Navbar title={t('rankings.title')} showBackButton backHref="/problems" />
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">{error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <PageTitle titleKey="pageTitle.rankings" />
      <Navbar title={t('rankings.title')} showBackButton backHref="/problems" />

      <div className="max-w-4xl mx-auto px-4 py-6">
        <div className="bg-white rounded-xl shadow-sm overflow-hidden">
          {/* Header */}
          <div className="bg-gradient-to-r from-indigo-500 to-purple-600 px-6 py-4">
            <h2 className="text-xl font-bold text-white">{t('rankings.title')}</h2>
          </div>

          {rankings.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              {t('rankings.empty')}
            </div>
          ) : (
            <div className="divide-y divide-gray-100">
              {/* Table header - hidden on mobile */}
              <div className="hidden sm:grid sm:grid-cols-12 gap-4 px-6 py-3 bg-gray-50 text-sm font-medium text-gray-500">
                <div className="col-span-1">{t('rankings.rank')}</div>
                <div className="col-span-7">{t('rankings.user')}</div>
                <div className="col-span-2 text-right">{t('rankings.likes')}</div>
                <div className="col-span-2 text-right">{t('rankings.comments')}</div>
              </div>

              {/* Rankings list */}
              {rankings.map((item, index) => (
                <div
                  key={item.userId}
                  className="px-4 sm:px-6 py-4 hover:bg-gray-50 transition cursor-pointer"
                  onClick={() => handleUserClick(item.userId)}
                >
                  {/* Mobile layout */}
                  <div className="flex sm:hidden items-center gap-3">
                    <span className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold ${getRankBadgeClass(index + 1)}`}>
                      {index + 1}
                    </span>
                    <img
                      src={getAvatarUrl(item.userId)}
                      alt="Avatar"
                      className="w-10 h-10 rounded-full object-cover border border-gray-200"
                      onError={(e) => {
                        (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                      }}
                    />
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-gray-900 truncate">{item.userName}</p>
                      <div className="flex gap-3 text-xs text-gray-500">
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
                      <span className={`inline-flex w-8 h-8 rounded-full items-center justify-center text-sm font-bold ${getRankBadgeClass(index + 1)}`}>
                        {index + 1}
                      </span>
                    </div>
                    <div className="col-span-7 flex items-center gap-3">
                      <img
                        src={getAvatarUrl(item.userId)}
                        alt="Avatar"
                        className="w-10 h-10 rounded-full object-cover border border-gray-200"
                        onError={(e) => {
                          (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%239CA3AF"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>';
                        }}
                      />
                      <span className="font-medium text-gray-900">{item.userName}</span>
                    </div>
                    <div className="col-span-2 text-right">
                      <span className="inline-flex items-center gap-1 text-gray-700">
                        <svg className="w-4 h-4 text-red-400" fill="currentColor" viewBox="0 0 24 24">
                          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
                        </svg>
                        {item.likes.toLocaleString()}
                      </span>
                    </div>
                    <div className="col-span-2 text-right text-gray-500">
                      {item.commentCount.toLocaleString()}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
