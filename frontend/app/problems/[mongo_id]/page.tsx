'use client';

import { useState, useEffect } from 'react';
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

                {/* Layout 4 */}
                <div className="bg-white rounded-lg shadow-sm p-6">
                  {/* Layout 4 Content */}
                </div>
              </div>
            </div>
          )}
        </main>
      </div>
    </>
  );
}
