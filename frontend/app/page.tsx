'use client';

import { useLanguage } from '@/contexts/LanguageContext';
import PageTitle from '@/components/PageTitle';
import Image from 'next/image';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';

export default function Home() {
  const { t } = useLanguage();
  const router = useRouter();
  const [isVisible, setIsVisible] = useState(false);
  const [backgroundOffset, setBackgroundOffset] = useState(0);

  useEffect(() => {
    // Trigger content animation after component mounts with a small delay
    const timer = setTimeout(() => {
      setIsVisible(true);
    }, 50);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    // Animate background image moving up
    const duration = 3000; // 3 seconds
    const startTime = Date.now();
    const maxOffset = 33; // 33% of image height

    const animate = () => {
      const elapsed = Date.now() - startTime;
      const progress = Math.min(elapsed / duration, 1);
      
      // Ease-out animation
      const easeOut = 1 - Math.pow(1 - progress, 3);
      const offset = easeOut * maxOffset;
      
      setBackgroundOffset(offset);

      if (progress < 1) {
        requestAnimationFrame(animate);
      }
    };

    const timer = setTimeout(() => {
      requestAnimationFrame(animate);
    }, 100);

    return () => clearTimeout(timer);
  }, []);

  return (
    <>
      <PageTitle titleKey="pageTitle.home" />
      <div className="min-h-screen flex items-center justify-center px-4 relative overflow-hidden">
        {/* Background Image with 70% opacity */}
        <div className="absolute inset-0 z-0 overflow-hidden">
          <div 
            className="absolute top-0 left-0 w-full transition-transform duration-[3000ms] ease-out"
            style={{ 
              height: '150%',
              transform: `translateY(-${backgroundOffset}%)` 
            }}
          >
            <Image
              src="/indexpage/le0ou.jpg"
              alt="Background"
              fill
              className="object-cover object-top opacity-80"
              priority
            />
          </div>
        </div>
        
        {/* Content Container */}
        <div className={`relative z-10 flex items-center gap-6 max-w-4xl w-full transition-all duration-1000 ease-out ${
          isVisible 
            ? 'opacity-100 scale-100 translate-y-0' 
            : 'opacity-0 scale-90 translate-y-4'
        }`}>
          {/* Avatar Image */}
          <div className="flex-shrink-0">
            <button
              onClick={() => router.push('/problems')}
              className="cursor-pointer hover:opacity-90 transition-opacity"
            >
              <div className="w-24 h-24 md:w-32 md:h-32 rounded-lg overflow-hidden shadow-lg border-2">
                <Image
                  src="/indexpage/le1ou.jpg"
                  alt="Avatar"
                  width={128}
                  height={128}
                  className="w-full h-full object-cover"
                  priority
                />
              </div>
            </button>
          </div>

          {/* Title and Subtitle */}
          <div className="flex flex-col">
            <h1 className="text-3xl md:text-4xl font-bold text-gray-900 dark:text-gray-100 mb-2 text-left">
              {t('home.title')}
            </h1>
            <p className="text-gray-600 dark:text-gray-300 text-left">{t('home.subtitle')}</p>
          </div>
        </div>
      </div>
    </>
  );
}

