'use client';

import { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter } from 'next/navigation';
import { problemAPI, searchAPI, SearchResult } from '@/lib/api';
import LanguageSelector from '@/components/LanguageSelector';
import PageTitle from '@/components/PageTitle';
import ColumnCustomizer, { ColumnConfig, ColumnId, DEFAULT_COLUMNS } from '@/components/ColumnCustomizer';

export default function ProblemsPage() {
  const { isAuthenticated, loading, logout, user } = useAuth();
  const { t } = useLanguage();
  const router = useRouter();

  // Upload states
  const [uploadMode, setUploadMode] = useState<'single' | 'multiple'>('single');
  const [singleUrl, setSingleUrl] = useState('');
  const [singleParsed, setSingleParsed] = useState<{ userId: string; taskId: string } | null>(null);
  const [multipleUrls, setMultipleUrls] = useState('');
  const [multipleParsed, setMultipleParsed] = useState<{
    pairs: Array<{ userId: string; taskId: string }>;
    userIdCount: number;
    taskIdCount: number;
    isValid: boolean;
  }>({ pairs: [], userIdCount: 0, taskIdCount: 0, isValid: false });
  const [uploadError, setUploadError] = useState('');
  const [uploadSuccess, setUploadSuccess] = useState('');
  const [uploadLoading, setUploadLoading] = useState(false);

  // Search states
  const [searchKeyword, setSearchKeyword] = useState('');
  const [currentSearchKeyword, setCurrentSearchKeyword] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searchTotal, setSearchTotal] = useState(0);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchError, setSearchError] = useState('');

  // Column customization states
  const [columns, setColumns] = useState<ColumnConfig[]>(DEFAULT_COLUMNS);
  const [isCustomizerOpen, setIsCustomizerOpen] = useState(false);

  useEffect(() => {
    if (!loading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, loading, router]);

  // Load recent problems on page load
  useEffect(() => {
    if (isAuthenticated && !loading) {
      const loadRecentProblems = async () => {
        setSearchLoading(true);
        setSearchError('');
        try {
          const response = await searchAPI.recent(15);
          setSearchResults(response.results);
          setSearchTotal(response.total);
          setCurrentSearchKeyword('');
        } catch (error: any) {
          setSearchError(error.message || t('problems.search.errorFailed'));
          setSearchResults([]);
          setSearchTotal(0);
        } finally {
          setSearchLoading(false);
        }
      };
      loadRecentProblems();
    }
  }, [isAuthenticated, loading, t]);

  // Load column configuration from localStorage
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('problemTableColumns');
      if (saved) {
        try {
          const parsed = JSON.parse(saved);
          setColumns(parsed);
        } catch (e) {
          console.error('Failed to parse saved column configuration', e);
        }
      }
    }
  }, []);

  // Save column configuration to localStorage
  const handleColumnsChange = (newColumns: ColumnConfig[]) => {
    // Ensure index is first and detail is last
    const sorted = [...newColumns].sort((a, b) => {
      if (a.id === 'index') return -1;
      if (b.id === 'index') return 1;
      if (a.id === 'detail') return 1;
      if (b.id === 'detail') return -1;
      return a.order - b.order;
    });
    
    const reordered = sorted.map((col, idx) => ({
      ...col,
      order: idx,
    }));
    
    setColumns(reordered);
    if (typeof window !== 'undefined') {
      localStorage.setItem('problemTableColumns', JSON.stringify(reordered));
    }
  };

  // Parse URL to extract userId and taskId
  const parseUrl = (url: string): { userId: string | null; taskId: string | null } => {
    try {
      const urlObj = new URL(url);
      const userId = urlObj.searchParams.get('userId');
      const taskId = urlObj.searchParams.get('encryptMockTaskNo');
      return { userId, taskId };
    } catch (error) {
      return { userId: null, taskId: null };
    }
  };

  // Parse single URL in real-time
  useEffect(() => {
    if (uploadMode === 'single' && singleUrl.trim()) {
      const parsed = parseUrl(singleUrl.trim());
      if (parsed.userId && parsed.taskId) {
        setSingleParsed({ userId: parsed.userId, taskId: parsed.taskId });
      } else {
        setSingleParsed(null);
      }
    } else {
      setSingleParsed(null);
    }
  }, [singleUrl, uploadMode]);

  // Parse multiple URLs in real-time
  useEffect(() => {
    if (uploadMode === 'multiple' && multipleUrls.trim()) {
      let urls: string[] = [];
      
      // Find all positions where URLs start (http:// or https://)
      // This handles both separated and concatenated URLs (no separators)
      const protocolPattern = /https?:\/\//gi;
      const matches: Array<{ index: number; protocol: string }> = [];
      let match;
      
      // Reset regex lastIndex to ensure we search from the beginning
      protocolPattern.lastIndex = 0;
      while ((match = protocolPattern.exec(multipleUrls)) !== null) {
        matches.push({ index: match.index, protocol: match[0] });
      }
      
      if (matches.length > 0) {
        // Extract URLs from each protocol start position
        for (let i = 0; i < matches.length; i++) {
          const start = matches[i].index;
          const nextStart = i < matches.length - 1 ? matches[i + 1].index : multipleUrls.length;
          
          // Extract the substring from current protocol to next protocol (or end)
          const urlSegment = multipleUrls.substring(start, nextStart);
          
          // Extract URL until whitespace/delimiter or next protocol
          // Match from protocol until whitespace, delimiter, or end
          const urlMatch = urlSegment.match(/^(https?:\/\/[^\s\n\t,;]+)/);
          if (urlMatch) {
            urls.push(urlMatch[1]);
          }
        }
      } else {
        // Fall back to delimiter-based splitting
        urls = multipleUrls
          .split(/[\n\t,;\s]+/)
          .map(url => url.trim())
          .filter(url => url.length > 0 && (url.startsWith('http://') || url.startsWith('https://')));
      }

      const userIds: string[] = [];
      const taskIds: string[] = [];
      const pairs: Array<{ userId: string; taskId: string }> = [];

      urls.forEach(url => {
        const parsed = parseUrl(url);
        if (parsed.userId) userIds.push(parsed.userId);
        if (parsed.taskId) taskIds.push(parsed.taskId);
        if (parsed.userId && parsed.taskId) {
          pairs.push({ userId: parsed.userId, taskId: parsed.taskId });
        }
      });

      // Remove duplicate pairs
      const uniquePairs = pairs.filter((pair, index, self) =>
        index === self.findIndex(p => p.userId === pair.userId && p.taskId === pair.taskId)
      );

      setMultipleParsed({
        pairs: uniquePairs,
        userIdCount: userIds.length,
        taskIdCount: taskIds.length,
        isValid: userIds.length === taskIds.length && uniquePairs.length > 0,
      });
    } else {
      setMultipleParsed({ pairs: [], userIdCount: 0, taskIdCount: 0, isValid: false });
    }
  }, [multipleUrls, uploadMode]);

  const handleSingleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    setUploadError('');
    setUploadSuccess('');
    
    if (!singleParsed) {
      setUploadError(t('problems.upload.errorInvalid'));
      return;
    }

    setUploadLoading(true);

    try {
      await problemAPI.upload({
        userId: singleParsed.userId,
        taskId: singleParsed.taskId,
      });
      setUploadSuccess(t('problems.upload.success'));
      setSingleUrl('');
      setSingleParsed(null);
    } catch (error: any) {
      setUploadError(error.message || t('problems.upload.error'));
    } finally {
      setUploadLoading(false);
    }
  };

  const handleMultipleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    setUploadError('');
    setUploadSuccess('');

    if (!multipleParsed.isValid) {
      setUploadError(t('problems.upload.errorInvalidData'));
      return;
    }

    if (multipleParsed.pairs.length === 0) {
      setUploadError(t('problems.upload.errorNoUrls'));
      return;
    }

    setUploadLoading(true);

    try {
      const response = await problemAPI.uploadMultiple({
        problems: multipleParsed.pairs,
      });
      setUploadSuccess(
        t('problems.upload.successMultiple', { success: response.success, failed: response.failed })
      );
      setMultipleUrls('');
      setMultipleParsed({ pairs: [], userIdCount: 0, taskIdCount: 0, isValid: false });
    } catch (error: any) {
      setUploadError(error.message || t('problems.upload.error'));
    } finally {
      setUploadLoading(false);
    }
  };

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!searchKeyword.trim()) {
      setSearchError(t('problems.search.error'));
      return;
    }

    const keyword = searchKeyword.trim();
    setSearchError('');
    setSearchLoading(true);

    try {
      const response = await searchAPI.search(keyword, 15);
      setSearchResults(response.results);
      setSearchTotal(response.total);
      setCurrentSearchKeyword(keyword);
      setSearchKeyword(''); // Clear the input after successful search
    } catch (error: any) {
      setSearchError(error.message || t('problems.search.errorFailed'));
      setSearchResults([]);
      setSearchTotal(0);
      setCurrentSearchKeyword('');
    } finally {
      setSearchLoading(false);
    }
  };

  const getProblemTitle = (result: SearchResult): string | JSX.Element => {
    // Check for highlight first
    if (result._highlight?.user_review && result._highlight.user_review.length > 0) {
      const highlighted = result._highlight.user_review[0];
      // Replace <mark> tags with styled version (black-red)
      const styledHtml = highlighted.replace(
        /<mark>(.*?)<\/mark>/g,
        '<mark style="background-color: transparent; color: #991b1b; font-weight: 600;">$1</mark>'
      );
      return <span dangerouslySetInnerHTML={{ __html: styledHtml }} />;
    }
    
    if (result.user_review && result.user_review.trim()) {
      return result.user_review;
    }
    return 'N/A';
  };

  const renderAnswerCell = (answer: number | null | undefined) => {
    if (answer === 1) {
      return (
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
          1
        </span>
      );
    } else if (answer === 2) {
      return (
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
          2
        </span>
      );
    }
    return <span className="text-gray-500">N/A</span>;
  };

  const formatTimestamp = (timestamp: number): string => {
    return new Date(timestamp * 1000).toLocaleString();
  };

  const formatRatio = (ratio1: number, ratio2: number): string => {
    return `${Math.round(ratio1)}-${Math.round(ratio2)}`;
  };

  const truncateTaskId = (taskId: string, maxLength: number = 80): string => {
    if (taskId.length <= maxLength) {
      return taskId;
    }
    return taskId.substring(0, maxLength) + '...';
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-gray-600">{t('loading')}</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <>
      <PageTitle titleKey="pageTitle.problems" />
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white shadow-sm border-b">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16 items-center">
            <div className="flex items-center">
              <h1 className="text-xl font-semibold text-gray-900">{t('problems.title')}</h1>
            </div>
            <div className="flex items-center space-x-2 sm:space-x-4">
              <LanguageSelector />
              <button
                onClick={() => router.push('/user')}
                className="text-sm text-indigo-600 hover:text-indigo-800"
              >
                {t('problems.userCenter')}
              </button>
              <span className="text-sm text-gray-700 hidden sm:inline">{t('problems.welcome', { username: user?.username || '' })}</span>
              <button
                onClick={logout}
                className="text-sm text-gray-600 hover:text-gray-800"
              >
                {t('logout')}
              </button>
            </div>
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Upload Section */}
        <div className="bg-white rounded-lg shadow-sm mb-8">
          <div className="p-6">
            <h2 className="text-2xl font-bold text-gray-900 mb-6">{t('problems.upload.title')}</h2>

            {/* Mode Toggle */}
            <div className="mb-6">
              <div className="flex space-x-4">
                <button
                  onClick={() => setUploadMode('single')}
                  className={`px-4 py-2 rounded-lg font-medium transition ${
                    uploadMode === 'single'
                      ? 'bg-indigo-600 text-white'
                      : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                  }`}
                >
                  {t('problems.upload.single')}
                </button>
                <button
                  onClick={() => setUploadMode('multiple')}
                  className={`px-4 py-2 rounded-lg font-medium transition ${
                    uploadMode === 'multiple'
                      ? 'bg-indigo-600 text-white'
                      : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                  }`}
                >
                  {t('problems.upload.multiple')}
                </button>
              </div>
            </div>

            {uploadError && (
              <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                <p className="text-sm text-red-600">{uploadError}</p>
              </div>
            )}

            {uploadSuccess && (
              <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-lg">
                <p className="text-sm text-green-600">{uploadSuccess}</p>
              </div>
            )}

            {/* Single Upload Form */}
            {uploadMode === 'single' && (
              <form onSubmit={handleSingleUpload} className="space-y-4">
                <div>
                  <label htmlFor="singleUrl" className="block text-sm font-medium text-gray-700 mb-2">
                    {t('problems.upload.url')}
                  </label>
                  <input
                    id="singleUrl"
                    type="text"
                    value={singleUrl}
                    onChange={(e) => setSingleUrl(e.target.value)}
                    required
                    className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
                    placeholder={t('problems.upload.urlPlaceholder')}
                  />
                  {singleParsed && (
                    <div className="mt-2 p-3 bg-gray-50 rounded-lg border border-gray-200">
                      <div className="text-sm">
                        <div className="mb-1 break-words">
                          <span className="font-medium text-gray-700">{t('problems.upload.userId')}: </span>
                          <span className="text-gray-900 break-all">{singleParsed.userId}</span>
                        </div>
                        <div className="break-words">
                          <span className="font-medium text-gray-700">{t('problems.upload.taskId')}: </span>
                          <span 
                            className="text-gray-900 break-all font-mono text-xs" 
                            title={singleParsed.taskId}
                          >
                            {truncateTaskId(singleParsed.taskId, 60)}
                          </span>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
                <button
                  type="submit"
                  disabled={uploadLoading || !singleParsed}
                  className="bg-indigo-600 text-white py-2 px-6 rounded-lg font-medium hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition"
                >
                  {uploadLoading ? t('problems.upload.submitting') : t('problems.upload.submit')}
                </button>
              </form>
            )}

            {/* Multiple Upload Form */}
            {uploadMode === 'multiple' && (
              <form onSubmit={handleMultipleUpload} className="space-y-4">
                <div>
                  <label htmlFor="multipleUrls" className="block text-sm font-medium text-gray-700 mb-2">
                    {t('problems.upload.multipleUrls')}
                  </label>
                  <textarea
                    id="multipleUrls"
                    value={multipleUrls}
                    onChange={(e) => setMultipleUrls(e.target.value)}
                    rows={8}
                    className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none font-mono text-sm"
                    placeholder={t('problems.upload.multipleUrlsPlaceholder')}
                  />
                  {multipleUrls.trim() && (
                    <div className="mt-2 p-3 bg-gray-50 rounded-lg border border-gray-200">
                      <div className="text-sm space-y-1">
                        <div>
                          <span className="font-medium text-gray-700">{t('problems.upload.userIdsFound')}: </span>
                          <span className={multipleParsed.userIdCount === multipleParsed.taskIdCount ? 'text-green-600' : 'text-red-600'}>
                            {multipleParsed.userIdCount}
                          </span>
                        </div>
                        <div>
                          <span className="font-medium text-gray-700">{t('problems.upload.taskIdsFound')}: </span>
                          <span className={multipleParsed.userIdCount === multipleParsed.taskIdCount ? 'text-green-600' : 'text-red-600'}>
                            {multipleParsed.taskIdCount}
                          </span>
                        </div>
                        <div>
                          <span className="font-medium text-gray-700">{t('problems.upload.validPairs')}: </span>
                          <span className={multipleParsed.isValid ? 'text-green-600' : 'text-red-600'}>
                            {multipleParsed.pairs.length}
                          </span>
                        </div>
                        {!multipleParsed.isValid && multipleParsed.userIdCount !== multipleParsed.taskIdCount && (
                          <div className="text-red-600 text-xs mt-2">
                            {t('problems.upload.warning')}
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
                <button
                  type="submit"
                  disabled={uploadLoading || !multipleParsed.isValid}
                  className="bg-indigo-600 text-white py-2 px-6 rounded-lg font-medium hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition"
                >
                  {uploadLoading ? t('problems.upload.submitting') : t('problems.upload.submitMultiple')}
                </button>
              </form>
            )}
          </div>
        </div>

        {/* Search Section */}
        <div className="bg-white rounded-lg shadow-sm">
          <div className="p-6">
            <h2 className="text-2xl font-bold text-gray-900 mb-6">{t('problems.search.title')}</h2>

            {/* Search Form */}
            <form onSubmit={handleSearch} className="mb-6">
              <div className="flex flex-col sm:flex-row gap-4">
                <input
                  type="text"
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  placeholder={t('problems.search.placeholder')}
                  className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
                />
                <button
                  type="button"
                  onClick={() => setIsCustomizerOpen(true)}
                  className="px-4 py-2 border border-gray-300 rounded-lg bg-white text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 transition whitespace-nowrap flex items-center justify-center"
                  title={t('problems.search.customizeColumns')}
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                  </svg>
                </button>
                <button
                  type="submit"
                  disabled={searchLoading}
                  className="bg-indigo-600 text-white py-2 px-6 rounded-lg font-medium hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition whitespace-nowrap"
                >
                  {searchLoading ? t('problems.search.searching') : t('problems.search.submit')}
                </button>
              </div>
            </form>

            {searchError && (
              <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                <p className="text-sm text-red-600">{searchError}</p>
              </div>
            )}

            {/* Search Results Header */}
            {currentSearchKeyword && (
              <div className="mb-4">
                <h3 className="text-lg font-semibold text-gray-900">
                  {t('problems.search.resultsFor', { keyword: currentSearchKeyword })}
                </h3>
                {searchResults.length > 0 && (
                  <p className="text-sm text-gray-600 mt-1">
                    {t('problems.search.found', { total: searchTotal, displayed: searchResults.length })}
                  </p>
                )}
              </div>
            )}

            {searchResults.length > 0 && (
              <div className="overflow-auto max-h-[600px] border border-gray-200 rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50 sticky top-0 z-10">
                    <tr>
                      {columns
                        .filter(col => col.visible)
                        .sort((a, b) => a.order - b.order)
                        .map((col) => (
                          <th
                            key={col.id}
                            className="px-2 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                          >
                            {col.id === 'index' && t('problems.search.index')}
                            {col.id === 'problemTitle' && t('problems.search.problemTitle')}
                            {col.id === 'time' && t('problems.search.time')}
                            {col.id === 'answer' && t('problems.search.answer')}
                            {col.id === 'ratio' && t('problems.search.ratio')}
                            {col.id === 'hot1' && t('problems.search.hot1')}
                            {col.id === 'comment' && t('problems.search.comment')}
                            {col.id === 'detail' && t('problems.search.detail')}
                          </th>
                        ))}
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {searchResults.map((result, index) => (
                      <tr key={index} className="hover:bg-gray-50">
                        {columns
                          .filter(col => col.visible)
                          .sort((a, b) => a.order - b.order)
                          .map((col) => {
                            if (col.id === 'index') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm text-gray-900">
                                  {index + 1}
                                </td>
                              );
                            }
                            if (col.id === 'problemTitle') {
                              return (
                                <td key={col.id} className="px-2 py-2 text-sm text-gray-900 max-w-xs truncate">
                                  {getProblemTitle(result)}
                                </td>
                              );
                            }
                            if (col.id === 'time') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm text-gray-500">
                                  {formatTimestamp(result.timestamp)}
                                </td>
                              );
                            }
                            if (col.id === 'answer') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm">
                                  {renderAnswerCell(result.answer)}
                                </td>
                              );
                            }
                            if (col.id === 'ratio') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm text-gray-900">
                                  {formatRatio(result.ratio_1, result.ratio_2)}
                                </td>
                              );
                            }
                            if (col.id === 'hot1') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm">
                                  {renderAnswerCell(result.hot1_answer)}
                                </td>
                              );
                            }
                            if (col.id === 'comment') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm">
                                  {renderAnswerCell(result.comment_answer)}
                                </td>
                              );
                            }
                            if (col.id === 'detail') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm">
                                  <a
                                    href={`/problems/${result.mongo_id}`}
                                    className="text-indigo-600 hover:text-indigo-800"
                                  >
                                    {t('problems.search.view')}
                                  </a>
                                </td>
                              );
                            }
                            return null;
                          })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {!searchLoading && searchResults.length === 0 && searchKeyword && (
              <div className="text-center py-8 text-gray-500">
                {t('problems.search.noResults')}
              </div>
            )}
          </div>
        </div>
      </main>

      <ColumnCustomizer
        isOpen={isCustomizerOpen}
        onClose={() => setIsCustomizerOpen(false)}
        columns={columns}
        onChange={handleColumnsChange}
      />
    </div>
    </>
  );
}
