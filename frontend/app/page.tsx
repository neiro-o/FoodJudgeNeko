'use client';

import { useLanguage } from '@/contexts/LanguageContext';
import PageTitle from '@/components/PageTitle';

export default function Home() {
  const { t } = useLanguage();

  return (
    <>
      <PageTitle titleKey="pageTitle.home" />
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
          <h1 className="text-4xl font-bold text-gray-900 mb-4">{t('home.title')}</h1>
          <p className="text-gray-600">{t('home.subtitle')}</p>
        </div>
      </div>
    </>
  )
}

