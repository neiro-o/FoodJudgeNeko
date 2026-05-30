'use client';

import { useState, useEffect, useRef } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter, useSearchParams } from 'next/navigation';
import { problemAPI, searchAPI, SearchResult, NotesSearchItem } from '@/lib/api';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import ColumnCustomizer, { ColumnConfig, ColumnId, DEFAULT_COLUMNS } from '@/components/ColumnCustomizer';

export default function ProblemsPage() {
  const { isAuthenticated, loading } = useAuth();
  const { t } = useLanguage();
  const router = useRouter();
  const searchParams = useSearchParams();
  
  // Ref to track the last processed URL keyword to prevent duplicate API calls
  // Use a sentinel value distinct from null so that the initial empty-keyword load is not skipped
  const lastProcessedUrlKeyword = useRef<string | undefined>(undefined);

  // Upload states
  const [uploadMode, setUploadMode] = useState<'single' | 'multiple'>('single');
  const [singleUrl, setSingleUrl] = useState('');
  const [singleParsed, setSingleParsed] = useState<{ userId: string; taskId: string } | null>(null);
  const [singleDailyParsed, setSingleDailyParsed] = useState<{ userId: string; dateId: string } | null>(null);
  const [multipleUrls, setMultipleUrls] = useState('');
  const [uploadStateLoaded, setUploadStateLoaded] = useState(false);
  const [multipleParsed, setMultipleParsed] = useState<{
    pairs: Array<{ userId: string; taskId: string }>;
    userIdCount: number;
    taskIdCount: number;
    isValid: boolean;
  }>({ pairs: [], userIdCount: 0, taskIdCount: 0, isValid: false });
  const [uploadError, setUploadError] = useState('');
  const [uploadSuccess, setUploadSuccess] = useState('');
  const [uploadLoading, setUploadLoading] = useState(false);
  
  // Hint for repeated clicks on same mode
  const [modeClickCount, setModeClickCount] = useState(0);
  const [lastModeClickTime, setLastModeClickTime] = useState(0);
  const [showModeHint, setShowModeHint] = useState(false);

  // Search states
  const [searchKeyword, setSearchKeyword] = useState('');
  const [currentSearchKeyword, setCurrentSearchKeyword] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searchTotal, setSearchTotal] = useState(0);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchError, setSearchError] = useState('');

  // Notes search states
  const [notesSearchResults, setNotesSearchResults] = useState<NotesSearchItem[]>([]);
  const [notesSearchLoading, setNotesSearchLoading] = useState(false);

  // Count states
  const [counts, setCounts] = useState<{ elasticsearch: number; mongodb: number; redis: number } | null>(null);
  const [countsLoading, setCountsLoading] = useState(false);

  // Column customization states
  const [columns, setColumns] = useState<ColumnConfig[]>(DEFAULT_COLUMNS);
  const [isCustomizerOpen, setIsCustomizerOpen] = useState(false);
  const [resultLimit, setResultLimit] = useState(15);
  const [blockMaliciousComment, setBlockMaliciousComment] = useState(true);

  useEffect(() => {
    if (!loading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, loading, router]);

  // Load recent problems on page load or search from URL params
  useEffect(() => {
    if (isAuthenticated && !loading) {
      const urlKeyword = searchParams.get('q');
      const trimmedKeyword = urlKeyword?.trim() || null;
      
      // Skip if we've already processed this URL keyword to avoid duplicate API calls
      // lastProcessedUrlKeyword starts as `undefined` (never processed), so the first visit always runs.
      // After processing: keyword string for keyword searches, '' for no-keyword loads.
      const processedKey = trimmedKeyword ?? '';
      if (processedKey === lastProcessedUrlKeyword.current) {
        return;
      }
      
      // Get saved result limit from localStorage (since state may not be updated yet)
      let limit = 15;
      if (typeof window !== 'undefined') {
        const savedLimit = localStorage.getItem('problemResultLimit');
        if (savedLimit) {
          const parsed = parseInt(savedLimit, 10);
          if (!isNaN(parsed) && parsed >= 5 && parsed <= 20) {
            limit = parsed;
          }
        }
      }
      
      if (trimmedKeyword) {
        // Mark this keyword as processed
        lastProcessedUrlKeyword.current = trimmedKeyword;
        
        // Search with the keyword from URL
        const performSearch = async () => {
          setSearchLoading(true);
          setSearchError('');
          setNotesSearchResults([]);
          try {
            const response = await searchAPI.search(trimmedKeyword, limit);
            setSearchResults(response.results);
            setSearchTotal(response.total);
            setCurrentSearchKeyword(trimmedKeyword);
            // Also fetch notes search results
            try {
              const notesResponse = await searchAPI.notesSearch(trimmedKeyword, 5);
              setNotesSearchResults(notesResponse || []);
            } catch {
              setNotesSearchResults([]);
            }
          } catch (error: any) {
            setSearchError(error.message || t('problems.search.errorFailed'));
            setSearchResults([]);
            setSearchTotal(0);
            setCurrentSearchKeyword('');
            // Reset the ref on error so we can retry
            lastProcessedUrlKeyword.current = undefined;
          } finally {
            setSearchLoading(false);
          }
        };
        performSearch();
      } else {
        // Mark empty keyword as processed (use empty string to distinguish from initial undefined)
        lastProcessedUrlKeyword.current = '';
        
        // Load recent problems
        const loadRecentProblems = async () => {
          setSearchLoading(true);
          setSearchError('');
          try {
            const response = await searchAPI.recent(limit);
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
    }
  }, [isAuthenticated, loading, t, searchParams]);

  // Load counts on page load
  useEffect(() => {
    if (isAuthenticated && !loading) {
      const loadCounts = async () => {
        setCountsLoading(true);
        try {
          const response = await problemAPI.count();
          setCounts(response.counts);
        } catch (error: any) {
          console.error('Failed to load counts:', error);
          // Don't show error to user, just leave counts as null
        } finally {
          setCountsLoading(false);
        }
      };
      loadCounts();
    }
  }, [isAuthenticated, loading]);

  // Load column configuration and result limit from localStorage
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
      const savedLimit = localStorage.getItem('problemResultLimit');
      if (savedLimit) {
        const parsed = parseInt(savedLimit, 10);
        if (!isNaN(parsed) && parsed >= 5 && parsed <= 20) {
          setResultLimit(parsed);
        }
      }
      const savedBlockMalicious = localStorage.getItem('blockMaliciousComment');
      if (savedBlockMalicious !== null) {
        setBlockMaliciousComment(savedBlockMalicious !== 'false');
      }
    }
  }, []);

  // Load upload state from localStorage
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('problemUploadState');
      if (saved) {
        try {
          const parsed = JSON.parse(saved);
          if (parsed.uploadMode) setUploadMode(parsed.uploadMode);
          if (parsed.singleUrl) setSingleUrl(parsed.singleUrl);
          if (parsed.multipleUrls) setMultipleUrls(parsed.multipleUrls);
        } catch (e) {
          console.error('Failed to parse saved upload state', e);
        }
      }
      setUploadStateLoaded(true);
    }
  }, []);

  // Save upload state to localStorage
  useEffect(() => {
    if (typeof window !== 'undefined' && uploadStateLoaded) {
      localStorage.setItem('problemUploadState', JSON.stringify({
        uploadMode,
        singleUrl,
        multipleUrls,
      }));
    }
  }, [uploadMode, singleUrl, multipleUrls, uploadStateLoaded]);

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

  // Save result limit to localStorage and re-fetch results
  const handleResultLimitChange = async (limit: number) => {
    setResultLimit(limit);
    if (typeof window !== 'undefined') {
      localStorage.setItem('problemResultLimit', String(limit));
    }
    
    // Re-fetch results with new limit
    setSearchLoading(true);
    setSearchError('');
    try {
      if (currentSearchKeyword) {
        const response = await searchAPI.search(currentSearchKeyword, limit);
        setSearchResults(response.results);
        setSearchTotal(response.total);
      } else {
        const response = await searchAPI.recent(limit);
        setSearchResults(response.results);
        setSearchTotal(response.total);
      }
    } catch (error: any) {
      setSearchError(error.message || t('problems.search.errorFailed'));
    } finally {
      setSearchLoading(false);
    }
  };

  const handleBlockMaliciousCommentChange = (block: boolean) => {
    setBlockMaliciousComment(block);
    if (typeof window !== 'undefined') {
      localStorage.setItem('blockMaliciousComment', String(block));
    }
  };

  // Handle upload mode button click
  const handleModeClick = (mode: 'single' | 'multiple') => {
    const now = Date.now();
    const timeDiff = now - lastModeClickTime;
    
    if (mode === uploadMode) {
      // Clicking the already selected mode
      if (timeDiff < 2000) {
        // Within 2 seconds of last click
        const newCount = modeClickCount + 1;
        setModeClickCount(newCount);
        if (newCount >= 2) {
          // Show hint after 2+ consecutive clicks
          setShowModeHint(true);
          setTimeout(() => setShowModeHint(false), 3000);
        }
      } else {
        setModeClickCount(1);
      }
      setLastModeClickTime(now);
    } else {
      // Switching mode
      setUploadMode(mode);
      setModeClickCount(0);
      setLastModeClickTime(0);
      setShowModeHint(false);
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

  // Parse daily report URL to extract shareUserId and dailyReportTime
  const parseDailyUrl = (url: string): { userId: string | null; dateId: string | null } => {
    try {
      const urlObj = new URL(url);
      const jumpScene = urlObj.searchParams.get('jumpScene');
      
      // Only parse if it's a daily report URL
      if (jumpScene !== 'dailyReport') {
        return { userId: null, dateId: null };
      }
      
      const userId = urlObj.searchParams.get('shareUserId');
      const dateId = urlObj.searchParams.get('dailyReportTime');
      return { userId, dateId };
    } catch (error) {
      return { userId: null, dateId: null };
    }
  };

  // Parse single URL in real-time
  useEffect(() => {
    if (uploadMode === 'single' && singleUrl.trim()) {
      // First check if it's a daily report URL
      const dailyParsed = parseDailyUrl(singleUrl.trim());
      if (dailyParsed.userId && dailyParsed.dateId) {
        setSingleDailyParsed({ userId: dailyParsed.userId, dateId: dailyParsed.dateId });
        setSingleParsed(null);
      } else {
        // Try regular URL parsing
        const parsed = parseUrl(singleUrl.trim());
        if (parsed.userId && parsed.taskId) {
          setSingleParsed({ userId: parsed.userId, taskId: parsed.taskId });
          setSingleDailyParsed(null);
        } else {
          setSingleParsed(null);
          setSingleDailyParsed(null);
        }
      }
    } else {
      setSingleParsed(null);
      setSingleDailyParsed(null);
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
    
    // Check if it's a daily report URL
    if (singleDailyParsed) {
      setUploadLoading(true);
      try {
        await problemAPI.uploadDaily({
          userId: singleDailyParsed.userId,
          dateId: singleDailyParsed.dateId,
        });
        setUploadSuccess(t('problems.upload.success'));
        setSingleUrl('');
        setSingleDailyParsed(null);
      } catch (error: any) {
        setUploadError(error.message || t('problems.upload.error'));
      } finally {
        setUploadLoading(false);
      }
      return;
    }
    
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
      const successCount = response.success ?? 0;
      const failedCount = response.failed ?? 0;
      setUploadSuccess(
        t('problems.upload.successMultiple', { success: successCount, failed: failedCount })
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
    setNotesSearchLoading(true);
    setNotesSearchResults([]);

    // Mark as processed before any await to prevent useEffect from re-triggering
    lastProcessedUrlKeyword.current = keyword;

    try {
      const response = await searchAPI.search(keyword, resultLimit);
      setSearchResults(response.results);
      setSearchTotal(response.total);
      setCurrentSearchKeyword(keyword);
      setSearchKeyword(''); // Clear the input after successful search
      
      // Update URL with search keyword
      const params = new URLSearchParams(searchParams.toString());
      params.set('q', keyword);
      router.push(`/problems?${params.toString()}`);

      // Fetch notes search results
      try {
        const notesResponse = await searchAPI.notesSearch(keyword, 5);
        setNotesSearchResults(notesResponse || []);
      } catch (notesError) {
        // Silently fail for notes search, don't show error
        console.error('Notes search failed:', notesError);
        setNotesSearchResults([]);
      }
    } catch (error: any) {
      setSearchError(error.message || t('problems.search.errorFailed'));
      setSearchResults([]);
      setSearchTotal(0);
      setCurrentSearchKeyword('');
      // Reset the ref on error so we can retry
      lastProcessedUrlKeyword.current = undefined;
    } finally {
      setSearchLoading(false);
      setNotesSearchLoading(false);
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
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300">
          1
        </span>
      );
    } else if (answer === 2) {
      return (
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
          2
        </span>
      );
    }
    return <span className="text-gray-500 dark:text-gray-400">N/A</span>;
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
      <div className="min-h-screen flex items-center justify-center dark:bg-gray-950">
        <div className="text-gray-600 dark:text-gray-300">{t('loading')}</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <>
      <PageTitle titleKey="pageTitle.problems" />
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
        <Navbar title={t('problems.title')} />

        <main className="max-w-7xl mx-auto px-2 sm:px-6 lg:px-8 py-8">
        {/* Upload Section */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow-sm mb-8">
          <div className="p-4 sm:p-6">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-6">{t('problems.upload.title')}</h2>

            {/* Mode Toggle */}
            <div className="mb-6">
              <div className="flex space-x-4">
                <button
                  onClick={() => handleModeClick('single')}
                  className={`px-4 py-2 rounded-lg font-medium transition ${
                    uploadMode === 'single'
                      ? 'bg-indigo-600 text-white'
                      : 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700'
                  }`}
                >
                  {t('problems.upload.single')}
                </button>
                <button
                  onClick={() => handleModeClick('multiple')}
                  className={`px-4 py-2 rounded-lg font-medium transition ${
                    uploadMode === 'multiple'
                      ? 'bg-indigo-600 text-white'
                      : 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700'
                  }`}
                >
                  {t('problems.upload.multiple')}
                </button>
              </div>
              
              {/* Hint for repeated clicks */}
              {showModeHint && (
                <div className="mt-2 p-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg animate-pulse">
                  <p className="text-sm text-amber-700 dark:text-amber-300">
                    {t('problems.upload.modeHint')}
                  </p>
                </div>
              )}
            </div>

            {uploadError && (
              <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-700 rounded-lg">
                <p className="text-sm text-red-600 dark:text-red-400">{uploadError}</p>
              </div>
            )}

            {uploadSuccess && (
              <div className="mb-4 p-3 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-700 rounded-lg">
                <p className="text-sm text-green-600 dark:text-green-400">{uploadSuccess}</p>
              </div>
            )}

            {/* Single Upload Form */}
            {uploadMode === 'single' && (
              <form onSubmit={handleSingleUpload} className="space-y-4">
                <div>
                  <label htmlFor="singleUrl" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    {t('problems.upload.url')}
                  </label>
                  <input
                    id="singleUrl"
                    type="text"
                    value={singleUrl}
                    onChange={(e) => setSingleUrl(e.target.value)}
                    required
                    className="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
                    placeholder={t('problems.upload.urlPlaceholder')}
                  />
                  {singleDailyParsed && (
                    <div className="mt-2 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-700">
                      <div className="text-sm">
                        <div className="mb-1 text-blue-800 dark:text-blue-300 font-medium">Daily Report Detected</div>
                        <div className="mb-1 break-words">
                          <span className="font-medium text-gray-700 dark:text-gray-300">User ID: </span>
                          <span className="text-gray-900 dark:text-gray-100 break-all">{singleDailyParsed.userId}</span>
                        </div>
                        <div className="break-words">
                          <span className="font-medium text-gray-700 dark:text-gray-300">Date ID: </span>
                          <span className="text-gray-900 dark:text-gray-100 break-all">{singleDailyParsed.dateId}</span>
                        </div>
                      </div>
                    </div>
                  )}
                  {singleParsed && !singleDailyParsed && (
                    <div className="mt-2 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                      <div className="text-sm">
                        <div className="mb-1 break-words">
                          <span className="font-medium text-gray-700 dark:text-gray-300">{t('problems.upload.userId')}: </span>
                          <span className="text-gray-900 dark:text-gray-100 break-all">{singleParsed.userId}</span>
                        </div>
                        <div className="break-words">
                          <span className="font-medium text-gray-700 dark:text-gray-300">{t('problems.upload.taskId')}: </span>
                          <span 
                            className="text-gray-900 dark:text-gray-100 break-all font-mono text-xs" 
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
                  disabled={uploadLoading || (!singleParsed && !singleDailyParsed)}
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
                  <label htmlFor="multipleUrls" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    {t('problems.upload.multipleUrls')}
                  </label>
                  <textarea
                    id="multipleUrls"
                    value={multipleUrls}
                    onChange={(e) => setMultipleUrls(e.target.value)}
                    rows={8}
                    className="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none font-mono text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
                    placeholder={t('problems.upload.multipleUrlsPlaceholder')}
                  />
                  {multipleUrls.trim() && (
                    <div className="mt-2 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                      <div className="text-sm space-y-1">
                        <div>
                          <span className="font-medium text-gray-700 dark:text-gray-300">{t('problems.upload.userIdsFound')}: </span>
                          <span className={multipleParsed.userIdCount === multipleParsed.taskIdCount ? 'text-green-600' : 'text-red-600'}>
                            {multipleParsed.userIdCount}
                          </span>
                        </div>
                        <div>
                          <span className="font-medium text-gray-700 dark:text-gray-300">{t('problems.upload.taskIdsFound')}: </span>
                          <span className={multipleParsed.userIdCount === multipleParsed.taskIdCount ? 'text-green-600' : 'text-red-600'}>
                            {multipleParsed.taskIdCount}
                          </span>
                        </div>
                        <div>
                          <span className="font-medium text-gray-700 dark:text-gray-300">{t('problems.upload.validPairs')}: </span>
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
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow-sm">
          <div className="p-4 sm:p-6">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-6">{t('problems.search.title')}</h2>

            {/* Search Form */}
            <form onSubmit={handleSearch} className="mb-6">
              <div className="flex flex-row gap-2 sm:gap-4">
                <input
                  type="text"
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  placeholder={t('problems.search.placeholder')}
                  className="flex-1 min-w-0 px-2 sm:px-4 py-2 text-sm sm:text-base border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
                />
                <button
                  type="button"
                  onClick={() => setIsCustomizerOpen(true)}
                  className="px-2 sm:px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 transition whitespace-nowrap flex items-center justify-center flex-shrink-0"
                  title={t('problems.search.customizeColumns')}
                >
                  <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                  </svg>
                </button>
                <button
                  type="submit"
                  disabled={searchLoading}
                  className="bg-indigo-600 text-white py-2 px-3 sm:px-6 rounded-lg font-medium hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition whitespace-nowrap flex-shrink-0 text-sm sm:text-base"
                >
                  {searchLoading ? t('problems.search.searching') : t('problems.search.submit')}
                </button>
              </div>
            </form>

            {searchError && (
              <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-700 rounded-lg">
                <p className="text-sm text-red-600 dark:text-red-400">{searchError}</p>
              </div>
            )}

            {/* Counts Display */}
            {(counts && (!currentSearchKeyword)) && (
              <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700 rounded-lg">
                <p className="text-sm text-blue-800 dark:text-blue-300">
                  {t('problems.search.counts', {
                    elasticsearch: counts.elasticsearch,
                    redis: counts.redis,
                    mongodb: counts.mongodb,
                  })}
                </p>
              </div>
            )}

            {/* Search Results Header */}
            {currentSearchKeyword && (
              <div className="mb-4">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                  {t('problems.search.resultsFor', { keyword: currentSearchKeyword })}
                </h3>
                {searchResults.length > 0 && (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
                    {t('problems.search.found', { total: searchTotal, displayed: searchResults.length })}
                  </p>
                )}
              </div>
            )}

            {searchResults.length > 0 && (
              <div className="overflow-x-auto -mx-2 sm:mx-0 max-h-[600px] border border-gray-200 dark:border-gray-700 rounded-lg">
                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                  <thead className="bg-gray-50 dark:bg-gray-800 sticky top-0 z-10">
                    <tr>
                      {columns
                        .filter(col => col.visible)
                        .sort((a, b) => a.order - b.order)
                        .map((col) => (
                          <th
                            key={col.id}
                            className="px-2 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
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
                  <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                    {searchResults.map((result, index) => (
                      <tr key={index} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                        {columns
                          .filter(col => col.visible)
                          .sort((a, b) => a.order - b.order)
                          .map((col) => {
                            if (col.id === 'index') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
                                  {index + 1}
                                </td>
                              );
                            }
                            if (col.id === 'problemTitle') {
                              return (
                                <td key={col.id} className="px-2 py-2 text-sm text-gray-900 dark:text-gray-100 max-w-xs truncate">
                                  {getProblemTitle(result)}
                                </td>
                              );
                            }
                            if (col.id === 'time') {
                              return (
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
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
                                <td key={col.id} className="px-2 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
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
                                    className="text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300"
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
              <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                {t('problems.search.noResults')}
              </div>
            )}

            {notesSearchResults.length > 0 && (
              <div className="mt-8">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
                  {t('problems.search.otherNotesTitle')}
                </h3>
                <div className="overflow-x-auto -mx-2 sm:mx-0 max-h-[600px] border border-gray-200 dark:border-gray-700 rounded-lg">
                  <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                    <thead className="bg-gray-50 dark:bg-gray-800 sticky top-0 z-10">
                      <tr>
                        <th className="px-2 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          {t('problems.search.index')}
                        </th>
                        <th className="px-2 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          {t('problems.search.problemTitle')}
                        </th>
                        <th className="px-2 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          {t('problems.search.answer')}
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                      {notesSearchResults.map((item, index) => (
                        <tr key={index} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                          <td className="px-2 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
                            {index + 1}
                          </td>
                          <td className="px-2 py-2 text-sm text-gray-900 dark:text-gray-100 max-w-xs truncate">
                            {item.text}
                          </td>
                          <td className="px-2 py-2 whitespace-nowrap text-sm">
                            {renderAnswerCell(item.answer)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
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
        resultLimit={resultLimit}
        onResultLimitChange={handleResultLimitChange}
        blockMaliciousComment={blockMaliciousComment}
        onBlockMaliciousCommentChange={handleBlockMaliciousCommentChange}
      />
    </div>
    </>
  );
}
