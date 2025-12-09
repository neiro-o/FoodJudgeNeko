'use client';

import { useEffect } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';

interface PageTitleProps {
  titleKey: string;
}

export default function PageTitle({ titleKey }: PageTitleProps) {
  const { t } = useLanguage();

  useEffect(() => {
    const title = t(titleKey);
    document.title = title;
  }, [titleKey, t]);

  return null;
}
