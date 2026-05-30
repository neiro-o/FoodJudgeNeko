'use client';

import { useLanguage } from '@/contexts/LanguageContext';

export default function LanguageSelector() {
  const { language, setLanguage, t } = useLanguage();

  return (
    <div className="flex items-center space-x-2">
      <span className="text-sm text-gray-600 dark:text-gray-300 hidden sm:inline">{t('language')}:</span>
      <select
        value={language}
        onChange={(e) => setLanguage(e.target.value as 'en' | 'zh')}
        className="text-sm px-2 py-1 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition cursor-pointer"
      >
        <option value="en">{t('english')}</option>
        <option value="zh">{t('chinese')}</option>
      </select>
    </div>
  );
}
