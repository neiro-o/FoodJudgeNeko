'use client';

import { useLanguage } from '@/contexts/LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';
import { useAuth } from '@/contexts/AuthContext';
import PageTitle from '@/components/PageTitle';
import AssetPreloader from '@/components/AssetPreloader';
import Link from 'next/link';

const mascots = ['classic', 'reader', 'computer', 'yellow-dress', 'black-dress', 'summer'];

export default function Home() {
  const { t } = useLanguage();
  const { isDark, setMode } = useTheme();
  const { isAuthenticated } = useAuth();
  const destination = isAuthenticated ? '/problems' : '/login';
  return (
    <>
      <PageTitle titleKey="pageTitle.home" />
      <AssetPreloader>
        <main className="home-scene">
          <div className="home-atmosphere" aria-hidden="true"><i /><i /><i /></div>
          <div className="home-mascots" aria-hidden="true">
            {mascots.map((name, index) => <div key={name} className={`home-mascot home-mascot-${index + 1}`}><img src={`/landing/${name}.png`} alt="" draggable={false} /></div>)}
          </div>
          <header className="home-header"><span>掉心心 <i>·</i> 你的胆子真是肥嘟嘟的！</span><button onClick={() => setMode(isDark ? 'light' : 'dark')} aria-label={isDark ? '切换到日间模式' : '切换到夜间模式'}>{isDark ? '☀' : '☾'}</button></header>
          <section className="home-intro">
            <Link href={destination} className="home-avatar" aria-label={isAuthenticated ? '进入题目页' : '进入登录页'}><img src="/indexpage/le1ou.jpg" width={144} height={144} alt="掉心心头像" /></Link>
            <div className="home-copy">
              <p className="home-eyebrow">哦哟，掉心心了！</p>
              <h1>{t('home.title')}<span aria-hidden="true">。</span></h1>
              <p className="home-subtitle">{t('home.subtitle')}</p>
              <Link href={destination} className="home-enter"> 进去瞧瞧 <span aria-hidden="true">↗</span></Link>
            </div>
          </section>
          <footer className="home-footer"><span className="home-status-dot" />缅怀老刘！永远为你应援~</footer>
        </main>
      </AssetPreloader>
    </>
  );
}
