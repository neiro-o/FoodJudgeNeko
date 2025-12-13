'use client';

import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter, useParams } from 'next/navigation';
import { searchAPI } from '@/lib/api';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import ProblemType1 from '@/components/ProblemType1';
import ProblemType2 from '@/components/ProblemType2';
import ProblemType3 from '@/components/ProblemType3';
import ProblemType4 from '@/components/ProblemType4';
import ProblemType5 from '@/components/ProblemType5';
import CommentList from '@/components/CommentList';

// Simple JSON to YAML converter
function jsonToYaml(obj: any, indent: number = 0): string {
  const spaces = '  '.repeat(indent);
  
  if (obj === null) return 'null';
  if (obj === undefined) return '';
  if (typeof obj === 'boolean') return obj ? 'true' : 'false';
  if (typeof obj === 'number') return String(obj);
  if (typeof obj === 'string') {
    // Check if string needs quoting
    if (obj.includes('\n') || obj.includes(':') || obj.includes('#') || 
        obj.includes("'") || obj.includes('"') || obj.trim() !== obj ||
        obj === '' || /^[\d.]+$/.test(obj) || ['true', 'false', 'null', 'yes', 'no'].includes(obj.toLowerCase())) {
      return JSON.stringify(obj);
    }
    return obj;
  }
  
  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]';
    return obj.map(item => {
      if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
        // For objects in array, format with - on first line, rest indented
        const entries = Object.entries(item);
        if (entries.length === 0) return `${spaces}- {}`;
        return entries.map(([key, value], idx) => {
          const prefix = idx === 0 ? `${spaces}- ` : `${spaces}  `;
          if (typeof value === 'object' && value !== null) {
            if (Array.isArray(value) && value.length > 0) {
              return `${prefix}${key}:\n${jsonToYaml(value, indent + 2)}`;
            } else if (!Array.isArray(value)) {
              return `${prefix}${key}:\n${jsonToYaml(value, indent + 2)}`;
            }
            return `${prefix}${key}: []`;
          }
          return `${prefix}${key}: ${jsonToYaml(value, indent + 1)}`;
        }).join('\n');
      }
      return `${spaces}- ${jsonToYaml(item, indent + 1)}`;
    }).join('\n');
  }
  
  if (typeof obj === 'object') {
    const entries = Object.entries(obj);
    if (entries.length === 0) return '{}';
    return entries.map(([key, value]) => {
      const yamlValue = jsonToYaml(value, indent + 1);
      if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
        return `${spaces}${key}:\n${yamlValue}`;
      } else if (Array.isArray(value) && value.length > 0) {
        return `${spaces}${key}:\n${yamlValue}`;
      }
      return `${spaces}${key}: ${yamlValue}`;
    }).join('\n');
  }
  
  return String(obj);
}

// Problem data interface
interface Reply {
  role: string;
  timestamp: number;
  content: string;
}

interface Appeal {
  role: string;
  timestamp: number;
  content: string;
  pics?: string[];
}

interface ProblemData {
  mongo_id: string;
  problem_type: number;
  user_review: string;
  review_pics?: string[];
  timestamp: number;
  replies?: Reply[];
  appeals?: Appeal[];
  // Add other fields as needed
  [key: string]: any;
}

export default function ProblemDetailPage() {
  const { isAuthenticated, loading } = useAuth();
  const { t } = useLanguage();
  const router = useRouter();
  const params = useParams();
  const mongoId = params.mongo_id as string;

  const [problem, setProblem] = useState<ProblemData | null>(null);
  const [loadingProblem, setLoadingProblem] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copyStatus, setCopyStatus] = useState<{ type: string; success: boolean } | null>(null);

  // Copy to clipboard helper
  const copyToClipboard = useCallback(async (text: string, type: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopyStatus({ type, success: true });
      setTimeout(() => setCopyStatus(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
      setCopyStatus({ type, success: false });
      setTimeout(() => setCopyStatus(null), 2000);
    }
  }, []);

  // Copy handlers
  const handleCopyLink = useCallback(() => {
    if (!problem) return;
    const link = `https://zqt.meituan.com/xiaomei/vote/jury/api/r/rediectByScene?jumpScene=mockTaskShare&userId=${problem.userId}&channel=mockTaskShare&encryptMockTaskNo=${problem.taskId}`;
    copyToClipboard(link, 'link');
  }, [problem, copyToClipboard]);

  const handleCopyJson = useCallback(() => {
    if (!problem) return;
    const jsonStr = JSON.stringify(problem, null, 2);
    copyToClipboard(jsonStr, 'json');
  }, [problem, copyToClipboard]);

  const handleCopyYaml = useCallback(() => {
    if (!problem) return;
    const yamlStr = jsonToYaml(problem);
    copyToClipboard(yamlStr, 'yaml');
  }, [problem, copyToClipboard]);

  useEffect(() => {
    if (!loading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, loading, router]);

  // Fetch problem data on page load
  useEffect(() => {
    if (isAuthenticated && !loading && mongoId) {
      const fetchProblem = async () => {
        setLoadingProblem(true);
        setError(null);
        try {
          const data = await searchAPI.getByMongoId(mongoId);
          setProblem(data);
        } catch (err: any) {
          console.error('Failed to fetch problem:', err);
          setError(err.message || 'Failed to fetch problem');
        } finally {
          setLoadingProblem(false);
        }
      };
      fetchProblem();
    }
  }, [isAuthenticated, loading, mongoId]);

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
        <Navbar title={t('problems.title')} showBackButton={true} backHref="/problems" />

        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          {/* Loading state */}
          {loadingProblem && (
            <div className="flex items-center justify-center py-12">
              <div className="text-gray-600">{t('loading')}</div>
            </div>
          )}

          {/* Error state */}
          {error && !loadingProblem && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4">
              <p className="text-red-600">{error}</p>
            </div>
          )}

          {/* Content */}
          {problem && !loadingProblem && (
            <div className="flex flex-col lg:flex-row gap-6">
              {/* Left Column (2/3 width on PC) */}
              <div className="w-full lg:w-2/3 flex flex-col gap-6">
                {/* Layout 1: Problem Timeline */}
                <div className="bg-white rounded-lg shadow-sm p-6">
                  {problem.problem_type === 1 && (
                    <ProblemType1
                      userReview={problem.user_review}
                      reviewPics={problem.review_pics}
                      timestamp={problem.timestamp}
                      replies={problem.replies}
                      appeals={problem.appeals}
                      orders={problem.orders}
                      orderDetail={problem.order_detail}
                      others={problem.others}
                      ratio1={problem.ratio_1}
                      ratio2={problem.ratio_2}
                      answer={problem.answer}
                    />
                  )}
                  {problem.problem_type === 2 && (
                    <ProblemType2
                      userReview={problem.user_review}
                      reviewPics={problem.review_pics}
                      timestamp={problem.timestamp}
                      replies={problem.replies}
                      appeals={problem.appeals}
                      others={problem.others}
                      ratio1={problem.ratio_1}
                      ratio2={problem.ratio_2}
                      answer={problem.answer}
                    />
                  )}
                  {problem.problem_type === 3 && (
                    <ProblemType3
                      appeals={problem.appeals}
                      orders={problem.orders}
                      orderDetail={problem.order_detail}
                      others={problem.others}
                      ratio1={problem.ratio_1}
                      ratio2={problem.ratio_2}
                      answer={problem.answer}
                    />
                  )}
                  {problem.problem_type === 4 && (
                    <ProblemType4
                      appeals={problem.appeals}
                      orderInfo={problem.order_info}
                      others={problem.others}
                      ratio1={problem.ratio_1}
                      ratio2={problem.ratio_2}
                      answer={problem.answer}
                    />
                  )}
                  {problem.problem_type === 5 && (
                    <ProblemType5
                      userReview={problem.user_review}
                      reviewPics={problem.review_pics}
                      timestamp={problem.timestamp}
                      ratio1={problem.ratio_1}
                      ratio2={problem.ratio_2}
                      answer={problem.answer}
                    />
                  )}
                </div>
              </div>

              {/* Right Column (1/3 width on PC) */}
              <div className="w-full lg:w-1/3 flex flex-col gap-6">
                {/* Layout 2: Comments */}
                <div className="bg-white rounded-lg shadow-sm p-6">
                  {problem.comments && (
                    <CommentList comments={problem.comments} />
                  )}
                </div>

                {/* Problem Operations */}
                <div className="bg-white rounded-lg shadow-sm p-6">
                  <h3 className="text-lg font-semibold text-gray-900 mb-4">
                    {t('problemOps.title')}
                  </h3>
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={handleCopyLink}
                      className={`px-4 py-2 rounded-lg text-sm font-medium transition ${
                        copyStatus?.type === 'link'
                          ? copyStatus.success
                            ? 'bg-green-100 text-green-700 border border-green-300'
                            : 'bg-red-100 text-red-700 border border-red-300'
                          : 'bg-indigo-50 text-indigo-700 hover:bg-indigo-100 border border-indigo-200'
                      }`}
                    >
                      {copyStatus?.type === 'link'
                        ? copyStatus.success
                          ? t('problemOps.copySuccess')
                          : t('problemOps.copyFailed')
                        : t('problemOps.copyLink')}
                    </button>
                    <button
                      onClick={handleCopyJson}
                      className={`px-4 py-2 rounded-lg text-sm font-medium transition ${
                        copyStatus?.type === 'json'
                          ? copyStatus.success
                            ? 'bg-green-100 text-green-700 border border-green-300'
                            : 'bg-red-100 text-red-700 border border-red-300'
                          : 'bg-indigo-50 text-indigo-700 hover:bg-indigo-100 border border-indigo-200'
                      }`}
                    >
                      {copyStatus?.type === 'json'
                        ? copyStatus.success
                          ? t('problemOps.copySuccess')
                          : t('problemOps.copyFailed')
                        : t('problemOps.copyJson')}
                    </button>
                    <button
                      onClick={handleCopyYaml}
                      className={`px-4 py-2 rounded-lg text-sm font-medium transition ${
                        copyStatus?.type === 'yaml'
                          ? copyStatus.success
                            ? 'bg-green-100 text-green-700 border border-green-300'
                            : 'bg-red-100 text-red-700 border border-red-300'
                          : 'bg-indigo-50 text-indigo-700 hover:bg-indigo-100 border border-indigo-200'
                      }`}
                    >
                      {copyStatus?.type === 'yaml'
                        ? copyStatus.success
                          ? t('problemOps.copySuccess')
                          : t('problemOps.copyFailed')
                        : t('problemOps.copyYaml')}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </main>
      </div>
    </>
  );
}
