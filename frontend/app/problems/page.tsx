'use client';

import { useState, useEffect, useRef } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter, useSearchParams } from 'next/navigation';
import { problemAPI, searchAPI, SearchResult, NotesSearchItem, ApiError, CODE_INSUFFICIENT_POINTS } from '@/lib/api';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import ColumnCustomizer, { ColumnConfig, ColumnId, DEFAULT_COLUMNS } from '@/components/ColumnCustomizer';
import Image from 'next/image';

const PREVIEW_RESULTS: SearchResult[] = [
  { id: 'preview-1', mongo_id: 'preview-1', user_review: '商家拒绝履行活动承诺，消费者申请退款是否应该支持？', timestamp: 1758090191, answer: 1, hot1_answer: 1, comment_ratio: 76, ratio_1: 57, ratio_2: 43, _score: 1 },
  { id: 'preview-2', mongo_id: 'preview-2', user_review: '外卖食品中吃出异物，无法提供完整证据时，平台应如何处理？', timestamp: 1758085527, answer: 2, hot1_answer: 2, comment_ratio: 61, ratio_1: 53, ratio_2: 47, _score: 1 },
  { id: 'preview-3', mongo_id: 'preview-3', user_review: '骑手未按备注要求送达，导致餐品变质，责任应由谁承担？', timestamp: 1758022928, answer: 1, hot1_answer: 1, comment_ratio: 50, ratio_1: 71, ratio_2: 29, _score: 1 },
  { id: 'preview-4', mongo_id: 'preview-4', user_review: '用户使用优惠券下单后，商家单方面取消订单，是否应赔付？', timestamp: 1757992653, answer: 2, hot1_answer: 2, comment_ratio: 24, ratio_1: 48, ratio_2: 52, _score: 1 },
];

const COUNT_METRICS = [
  { key: 'elasticsearch', icon: '📋', label: '题目总数' },
  { key: 'redis', icon: '📤', label: '上传队列' },
  { key: 'mongodb', icon: '📖', label: '原始数据' },
] as const;

export default function ProblemsPage() {
  const { isAuthenticated, loading, user } = useAuth();
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
  const [searchErrorIsInsufficientPoints, setSearchErrorIsInsufficientPoints] = useState(false);

  // Notes search states
  const [notesSearchResults, setNotesSearchResults] = useState<NotesSearchItem[]>([]);
  const [notesSearchLoading, setNotesSearchLoading] = useState(false);

  // Count states
  const [counts, setCounts] = useState<{ elasticsearch: number; mongodb: number; redis: number } | null>(null);
  const [countsLoading, setCountsLoading] = useState(false);
  const [countMetricIndex, setCountMetricIndex] = useState(0);

  // Column customization states
  const [columns, setColumns] = useState<ColumnConfig[]>(DEFAULT_COLUMNS);
  const [isCustomizerOpen, setIsCustomizerOpen] = useState(false);
  const [displayItems, setDisplayItems] = useState({ ratio: false, comments: false, ai: false });
  const [draftDisplayItems, setDraftDisplayItems] = useState(displayItems);
  useEffect(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('problemDisplayItemsV1') || '{}');
      setDisplayItems({ ratio: saved.ratio === true, comments: saved.comments === true, ai: saved.ai === true });
    } catch { /* Keep defaults if stored preferences are invalid. */ }
  }, []);
  const returnToSearch = () => window.location.assign('/problems');
  const [resultLimit, setResultLimit] = useState(15);
  const [blockMaliciousComment, setBlockMaliciousComment] = useState(true);
  const [draftResultLimit, setDraftResultLimit] = useState(15);
  const [draftBlockMaliciousComment, setDraftBlockMaliciousComment] = useState(true);
  const [isUploadOpen, setIsUploadOpen] = useState(false);

  useEffect(() => {
    if (!loading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, loading, router]);

  // Sets the search error message and flags whether it was caused by an
  // insufficient-points restriction so the UI can render a special hint.
  const applySearchError = (error: any) => {
    const isInsufficientPoints = error instanceof ApiError && error.code === CODE_INSUFFICIENT_POINTS;
    setSearchErrorIsInsufficientPoints(isInsufficientPoints);
    setSearchError(error.message || t('problems.search.errorFailed'));
  };

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
          setSearchErrorIsInsufficientPoints(false);
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
            applySearchError(error);
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
        // The initial page is intentionally a calm search landing state.
        // Recent problems are loaded only after the user requests them.
        lastProcessedUrlKeyword.current = '';
        setSearchResults([]);
        setSearchTotal(0);
        setCurrentSearchKeyword('');
      }
    }
  }, [isAuthenticated, loading, t, searchParams]);

  // Load counts on page load
  useEffect(() => {
    if (isAuthenticated && !loading && process.env.NEXT_PUBLIC_UI_PREVIEW !== '1') {
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
    setSearchErrorIsInsufficientPoints(false);
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
      applySearchError(error);
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

  const analyzeUnifiedUploads = (text: string) => {
    const urls = text.match(/https?:\/\/[^\s\n\t,;]+/gi) || [];
    const regular = urls.flatMap((url) => {
      const parsed = parseUrl(url);
      return parsed.userId && parsed.taskId ? [{ userId: parsed.userId, taskId: parsed.taskId }] : [];
    });
    const daily = urls.flatMap((url) => {
      const parsed = parseDailyUrl(url);
      return parsed.userId && parsed.dateId ? [{ userId: parsed.userId, dateId: parsed.dateId }] : [];
    });

    const uniqueRegular = regular.filter((item, index, items) =>
      index === items.findIndex((candidate) => candidate.userId === item.userId && candidate.taskId === item.taskId)
    );
    const uniqueDaily = daily.filter((item, index, items) =>
      index === items.findIndex((candidate) => candidate.userId === item.userId && candidate.dateId === item.dateId)
    );

    return {
      regular: uniqueRegular,
      daily: uniqueDaily,
      total: uniqueRegular.length + uniqueDaily.length,
      invalid: Math.max(0, urls.length - uniqueRegular.length - uniqueDaily.length),
    };
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

  const handleUnifiedUpload = async (event: React.FormEvent) => {
    event.preventDefault();
    setUploadError('');
    setUploadSuccess('');
    const analysis = analyzeUnifiedUploads(multipleUrls);
    if (analysis.total === 0) {
      setUploadError('没有识别到有效的普通题目链接或 daily 链接');
      return;
    }

    setUploadLoading(true);
    let success = 0;
    let failed = 0;
    try {
      if (analysis.regular.length === 1) {
        await problemAPI.upload(analysis.regular[0]);
        success += 1;
      } else if (analysis.regular.length > 1) {
        const result = await problemAPI.uploadMultiple({ problems: analysis.regular });
        success += result.success || 0;
        failed += result.failed || 0;
      }

      const dailyResults = await Promise.allSettled(
        analysis.daily.map((item) => problemAPI.uploadDaily(item))
      );
      success += dailyResults.filter((result) => result.status === 'fulfilled').length;
      failed += dailyResults.filter((result) => result.status === 'rejected').length;

      if (success > 0) setUploadSuccess(`上传完成：成功 ${success} 道${failed ? `，失败 ${failed} 道` : ''}`);
      if (failed > 0 && success === 0) setUploadError(`上传失败：${failed} 道，请检查链接或稍后重试`);
      if (failed === 0) setMultipleUrls('');
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
      setSearchErrorIsInsufficientPoints(false);
      return;
    }

    const keyword = searchKeyword.trim();
    setSearchError('');
    setSearchErrorIsInsufficientPoints(false);
    setSearchLoading(true);
    setNotesSearchLoading(true);
    setNotesSearchResults([]);

    // Mark as processed before any await to prevent useEffect from re-triggering
    lastProcessedUrlKeyword.current = keyword;

    try {
      if (process.env.NEXT_PUBLIC_UI_PREVIEW === '1') {
        await new Promise((resolve) => setTimeout(resolve, 650));
        setSearchResults(PREVIEW_RESULTS);
        setSearchTotal(PREVIEW_RESULTS.length);
        setCurrentSearchKeyword(keyword);
        setSearchKeyword(keyword);
        router.push(`/problems?q=${encodeURIComponent(keyword)}`);
        return;
      }
      const response = await searchAPI.search(keyword, resultLimit);
      setSearchResults(response.results);
      setSearchTotal(response.total);
      setCurrentSearchKeyword(keyword);
      setSearchKeyword(keyword);
      
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
      applySearchError(error);
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

  const handleRecentProblems = async () => {
    setSearchError('');
    setSearchErrorIsInsufficientPoints(false);
    setSearchLoading(true);
    setCurrentSearchKeyword('最近题目');
    setSearchKeyword('');
    try {
      if (process.env.NEXT_PUBLIC_UI_PREVIEW === '1') {
        await new Promise((resolve) => setTimeout(resolve, 650));
        setSearchResults(PREVIEW_RESULTS);
        setSearchTotal(PREVIEW_RESULTS.length);
        lastProcessedUrlKeyword.current = '';
        router.push('/problems?view=recent');
        return;
      }
      const response = await searchAPI.recent(resultLimit);
      setSearchResults(response.results);
      setSearchTotal(response.total);
      router.push('/problems?view=recent');
    } catch (error: any) {
      applySearchError(error);
      setSearchResults([]);
      setSearchTotal(0);
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
        <span className="answer-capsule answer-one">
          1
        </span>
      );
    } else if (answer === 2) {
      return (
        <span className="answer-capsule answer-two">
          2
        </span>
      );
    }
    return <span className="text-gray-500 dark:text-gray-400">N/A</span>;
  };

  const formatTimestamp = (timestamp: number): string => {
    return new Date(timestamp * 1000).toLocaleString();
  };

  const formatRatio = (ratio1: number, ratio2: number): JSX.Element => {
    return (
      <span className="ratio-colored">
        <b>{Math.round(ratio1)}</b><i>-</i><em>{Math.round(ratio2)}</em>
      </span>
    );
  };

  const renderCommentRatio = (ratio: number | null | undefined): JSX.Element => {
    if (ratio === null || ratio === undefined || !Number.isFinite(ratio)) {
      return <span className="text-gray-500 dark:text-gray-400">N/A</span>;
    }

    const value = Math.min(100, Math.max(0, ratio));
    const distanceFromMiddle = Math.abs(value - 50) / 50;
    const endpoint = value >= 50 ? [34, 197, 94] : [239, 68, 68];
    const channel = (target: number) => Math.round(255 + (target - 255) * distanceFromMiddle);
    const color = `rgb(${channel(endpoint[0])}, ${channel(endpoint[1])}, ${channel(endpoint[2])})`;

    return (
      <span className="comment-ratio" style={{ color }}>
        {Math.round(value)}%
      </span>
    );
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

  const unifiedUploadAnalysis = analyzeUnifiedUploads(multipleUrls);
  const legacyPage = (
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
              <div
                className={`mb-4 p-3 rounded-lg border ${
                  searchErrorIsInsufficientPoints
                    ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-300 dark:border-amber-700'
                    : 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-700'
                }`}
              >
                <p
                  className={`text-sm ${
                    searchErrorIsInsufficientPoints
                      ? 'text-amber-800 dark:text-amber-300 font-medium'
                      : 'text-red-600 dark:text-red-400'
                  }`}
                >
                  {searchErrorIsInsufficientPoints
                    ? `⚠️ ${t('problems.search.errorInsufficientPoints')}`
                    : searchError}
                </p>
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
                                  {renderCommentRatio(result.comment_ratio)}
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

  void legacyPage;
  const hasSearchState = searchLoading || Boolean(currentSearchKeyword) || searchResults.length > 0 || Boolean(searchError);
  const activeCountMetric = COUNT_METRICS[countMetricIndex];
  const activeCount = counts?.[activeCountMetric.key];

  return (
    <>
      <PageTitle titleKey="pageTitle.problems" />
      <div className={`brand-page search-page ${hasSearchState ? 'search-page-results' : 'search-page-landing'}`}>
        <Navbar title="美团评审团 · 题目答案搜索" />

        <main className="search-stage">
          <section className="search-landing-visual" aria-hidden={hasSearchState}>
            <div className="search-scribble search-scribble-left">人人不掉心，<br /><span>期期 105！</span></div>
            <div className="search-scribble search-scribble-right">打爆唐 B 评审！</div>
            <div className="search-mascot-wrap">
              <Image
                src="/brand/kangaroo-reader.png?v=2"
                alt="正在读题的袋鼠"
                width={1254}
                height={1254}
                priority
                unoptimized
                className="search-mascot"
              />
              <Image
                src="/brand/book-stack.png"
                unoptimized
                alt="真实题目、高分答案、少走弯路书堆"
                width={1698}
                height={926}
                className="search-book-stack"
                priority
              />
            </div>
            <Image
              src="/brand/kangaroo-milk-tea.png"
              unoptimized
              alt="袋鼠造型奶茶"
              width={1024}
              height={1536}
              className="search-milk-tea"
              priority
            />
            <div className="search-hero-copy">
              <p className="search-eyebrow">ANSWER FINDER · 题目答案搜索</p>
              <h1>小美搜题</h1>
              <p>打爆歪题，少掉歪心！</p>
            </div>
          </section>

          <section className="search-workspace">
            <form onSubmit={handleSearch} className="search-command" role="search">
              <input
                type="search"
                value={searchKeyword}
                onChange={(event) => setSearchKeyword(event.target.value)}
                placeholder="今天想搜点什么？"
                aria-label="搜索题目"
              />
              {!hasSearchState && <span className="search-hint">加空格带上日期可以精确搜索对应日期评价</span>}
              {searchKeyword && (
                <button
                  type="button"
                  className="search-clear"
                  onClick={() => setSearchKeyword('')}
                  aria-label="清空搜索内容"
                  title="清空"
                >
                  <span aria-hidden="true">×</span>
                </button>
              )}
              <button type="submit" disabled={searchLoading} aria-label="提交搜索">
                {searchLoading ? (
                  <span className="search-spinner search-spinner-small" />
                ) : (
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m21 3-7.2 18-3.9-7-6.9-4L21 3Z" /><path d="m10 14 4-4" /></svg>
                )}
              </button>
            </form>

            {!hasSearchState && (
              <div className="search-quick-actions">
                <button
                  type="button"
                  className="quick-pill count-switcher"
                  onClick={() => setCountMetricIndex((index) => (index + 1) % COUNT_METRICS.length)}
                  data-tooltip={activeCountMetric.label}
                  aria-label={`${activeCountMetric.label}：${activeCount ?? '加载中'}，点击查看下一项`}
                >
                  <span aria-hidden="true">{activeCountMetric.icon}</span>
                  <b>{countsLoading || activeCount === undefined ? '—' : activeCount}</b>
                </button>
                <button className="quick-pill" onClick={() => router.push('/points')}><span className="coin-icon">●</span>{user?.points ?? 0}</button>
                <button className="quick-pill" onClick={() => router.push('/points')}><span>▥</span> 查询分榜</button>
                <button className="quick-pill" onClick={() => router.push('/user_stats')}><span>♙</span> 查询用户</button>
                <button className="quick-pill" onClick={handleRecentProblems}><span>▤</span> 最近题目</button>
              </div>
            )}

            {hasSearchState && (
              <div className="search-results-shell brand-card">
                {searchLoading ? (
                  <div className="search-loading-state">
                    <span className="search-spinner" />
                    <strong>袋鼠增肥中……</strong>
                    <p>哦呦，掉小心心了！</p>
                  </div>
                ) : (
                  <>
                    <div className="search-results-heading">
                      <div>
                        <span>SEARCH RESULTS</span>
                        <h2>共找到 <b>{searchTotal}</b> 道题，显示前 <b>{searchResults.length}</b> 个</h2>
                      </div>
                      <div className="result-heading-actions">
                      <button onClick={() => { setDraftDisplayItems(displayItems); setDraftResultLimit(resultLimit); setDraftBlockMaliciousComment(blockMaliciousComment); setIsCustomizerOpen(true); }} className="result-settings" aria-label="选择显示项">
                        <svg viewBox="0 0 24 24"><path d="M4 7h10M18 7h2M4 17h2m4 0h10M14 4v6M6 14v6" /></svg>
                      </button>
                      <button type="button" className="result-settings" onClick={returnToSearch} aria-label="关闭结果，返回搜题" title="关闭结果，返回搜题">
                        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
                      </button>
                      </div>
                    </div>

                    {searchError && (
                      <div className={`search-alert ${searchErrorIsInsufficientPoints ? 'warning' : 'error'}`}>
                        {searchErrorIsInsufficientPoints ? `⚠️ ${t('problems.search.errorInsufficientPoints')}` : searchError}
                      </div>
                    )}

                    {!searchError && searchResults.length === 0 && (
                      <div className="search-empty">没有找到相关题目，换个关键词试试吧。</div>
                    )}

                    <div className="result-card-list">
                      {searchResults.map((result, index) => (
                        <a
                          href={`/problems/${result.mongo_id}`}
                          className="result-card"
                          key={result.mongo_id || index}
                          style={{ animationDelay: `${Math.min(index, 10) * 75}ms` }}
                        >
                          <span className="result-index">{index + 1}</span>
                          <div className="result-main">
                            <h3>{getProblemTitle(result)}</h3>
                            <p>上传时间：{formatTimestamp(result.timestamp)}</p>
                          </div>
                          <div className="result-metrics" data-count={1 + Object.values(displayItems).filter(Boolean).length}>
                            <span>答案 {renderAnswerCell(result.answer)}</span>
                            {displayItems.ratio && <span>比例 <em>{formatRatio(result.ratio_1, result.ratio_2)}</em></span>}
                            {displayItems.comments && <span>评论区比例 {renderCommentRatio(result.comment_ratio)}</span>}
                            {displayItems.ai && <span>AI 判断 {renderAnswerCell(result.hot1_answer)}</span>}
                          </div>
                        </a>
                      ))}
                    </div>

                    {notesSearchResults.length > 0 && (
                      <div className="notes-results">
                        <h3>{t('problems.search.otherNotesTitle')}</h3>
                        {notesSearchResults.map((item, index) => (
                          <div key={`${item.text}-${index}`}><span>{index + 1}</span><p>{item.text}</p>{renderAnswerCell(item.answer)}</div>
                        ))}
                      </div>
                    )}
                  </>
                )}
              </div>
            )}
          </section>
        </main>

        {!hasSearchState && <p className="search-footer-note">让每一道题，<br />都有更好的答案！</p>}
        <button className="upload-fab" onClick={() => setIsUploadOpen((open) => !open)} aria-label="上传题目">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        </button>

        {isUploadOpen && (
          <section className="upload-popover brand-card" role="dialog" aria-label="上传题目">
              <header>
                <div><span>CONTRIBUTE</span><h2>{t('problems.upload.title')}</h2></div>
                <button onClick={() => setIsUploadOpen(false)} aria-label="关闭">×</button>
              </header>
              {uploadError && <div className="search-alert error">{uploadError}</div>}
              {uploadSuccess && <div className="search-alert success">{uploadSuccess}</div>}
              <form onSubmit={handleUnifiedUpload} className="upload-form">
                <label htmlFor="unifiedUrls">粘贴题目链接</label>
                <textarea id="unifiedUrls" value={multipleUrls} onChange={(event) => setMultipleUrls(event.target.value)} placeholder="支持普通题目链接和 daily 链接；多条链接可换行粘贴" rows={5} autoFocus />
                <div className="upload-detection-row">
                  <span>普通题目 <b>{unifiedUploadAnalysis.regular.length}</b></span>
                  <span>Daily <b>{unifiedUploadAnalysis.daily.length}</b></span>
                  {unifiedUploadAnalysis.invalid > 0 && <span className="invalid">未识别 {unifiedUploadAnalysis.invalid}</span>}
                </div>
                <button className="brand-primary-button" type="submit" disabled={uploadLoading || unifiedUploadAnalysis.total === 0}>
                  {uploadLoading ? t('problems.upload.submitting') : unifiedUploadAnalysis.total > 0 ? `上传 ${unifiedUploadAnalysis.total} 道题` : '上传题目'}
                </button>
              </form>
          </section>
        )}

        {isCustomizerOpen && (
          <div className="display-settings-overlay" onClick={() => setIsCustomizerOpen(false)}>
            <section className="display-settings brand-card" role="dialog" aria-modal="true" aria-labelledby="display-settings-title" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => { if (event.key === 'Escape') setIsCustomizerOpen(false); }}>
              <h2 id="display-settings-title">选择显示项</h2>
              <p>答案始终显示，以下信息可按需开启。</p>
              {([['ratio', '比例'], ['comments', '评论区比例'], ['ai', 'AI 判断']] as const).map(([key, label]) => (
                <label key={key}><span>{label}</span><input type="checkbox" checked={draftDisplayItems[key]} onChange={(event) => setDraftDisplayItems({ ...draftDisplayItems, [key]: event.target.checked })} /></label>
              ))}
              <div className="display-settings-section">
                <h3>搜索设置</h3>
                <label>
                  <span><b>每页显示条数</b><small>设置搜索结果一次显示的数量</small></span>
                  <span className="display-limit-control">
                    <input type="range" min="5" max="20" value={draftResultLimit} onChange={(event) => setDraftResultLimit(Number(event.target.value))} />
                    <output>{draftResultLimit}</output>
                  </span>
                </label>
                <label>
                  <span><b>屏蔽恶意用户</b><small>隐藏疑似恶意用户产生的内容</small></span>
                  <span className="display-toggle-text">
                    <input type="checkbox" checked={draftBlockMaliciousComment} onChange={(event) => setDraftBlockMaliciousComment(event.target.checked)} />
                    {draftBlockMaliciousComment ? '开启' : '关闭'}
                  </span>
                </label>
              </div>
              <footer>
                <button className="quick-pill" autoFocus onClick={() => setIsCustomizerOpen(false)}>取消</button>
                <button className="brand-primary-button" onClick={() => {
                  localStorage.setItem('problemDisplayItemsV1', JSON.stringify(draftDisplayItems));
                  setDisplayItems(draftDisplayItems);
                  handleBlockMaliciousCommentChange(draftBlockMaliciousComment);
                  if (draftResultLimit !== resultLimit) void handleResultLimitChange(draftResultLimit);
                  setIsCustomizerOpen(false);
                }}>保存</button>
              </footer>
            </section>
          </div>
        )}
      </div>
    </>
  );
}
