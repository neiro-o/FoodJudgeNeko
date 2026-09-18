'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';
import BrandLogo from '@/components/BrandLogo';
import AccountAvatar from '@/components/AccountAvatar';

interface NavbarProps {
  title: string;
  showBackButton?: boolean;
  backHref?: string;
}

export default function Navbar({ title, showBackButton = false, backHref = '/problems' }: NavbarProps) {
  const { logout, user } = useAuth();
  const { language, setLanguage, t } = useLanguage();
  const { isDark, setMode } = useTheme();
  const router = useRouter();
  const [openMenu, setOpenMenu] = useState<'nav' | 'user' | null>(null);
  const navRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (navRef.current && !navRef.current.contains(event.target as Node)) setOpenMenu(null);
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, []);

  const navigate = (path: string) => {
    setOpenMenu(null);
    if (path === '/problems') {
      window.location.assign('/problems');
      return;
    }
    router.push(path);
  };

  return (
    <nav className="brand-navbar" ref={navRef}>
      <div className="brand-navbar-inner">
        <div className="brand-navbar-left">
          {showBackButton && (
            <button className="brand-icon-button brand-back" onClick={() => router.push(backHref)} aria-label={t('back') || '返回'}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6" /></svg>
            </button>
          )}
          <BrandLogo compact markImageSrc="/brand/navbar-logo.png" />
          <span className="brand-navbar-divider" />
          <button className="brand-navbar-title" onClick={() => setOpenMenu(openMenu === 'nav' ? null : 'nav')}>
            {title || '题目答案搜索'}
          </button>
          {openMenu === 'nav' && (
            <div className="brand-popover brand-nav-menu">
              <button onClick={() => navigate('/problems')}>搜索题目</button>
              <button onClick={() => navigate('/user_stats')}>查询用户</button>
              <button onClick={() => navigate('/points')}>积分榜</button>
              <button onClick={() => navigate('/user')}>用户中心</button>
            </div>
          )}
        </div>

        <div className="brand-navbar-actions">
          <button className="brand-icon-button" onClick={() => setLanguage(language === 'zh' ? 'en' : 'zh')} aria-label="切换语言">
            <span className="brand-language-label">{language === 'zh' ? '中' : 'EN'}</span>
          </button>
          <button className="brand-icon-button" onClick={() => setMode(isDark ? 'light' : 'dark')} aria-label="切换主题">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              {isDark ? <path d="M12 3v2m0 14v2m9-9h-2M5 12H3m15.4-6.4L17 7M7 17l-1.4 1.4M18.4 18.4 17 17M7 7 5.6 5.6M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z" /> : <path d="M20 15.2A8 8 0 0 1 8.8 4 8.5 8.5 0 1 0 20 15.2Z" />}
            </svg>
          </button>
          <div className="brand-user-wrap">
            <button className="brand-avatar-button" onClick={() => setOpenMenu(openMenu === 'user' ? null : 'user')} aria-label="用户菜单">
              {user?.id ? (
                <AccountAvatar
                  accountId={user.id}
                  username={user.username}
                  sizeClassName="brand-avatar-image"
                />
              ) : (
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm7 8a7 7 0 0 0-14 0" /></svg>
              )}
            </button>
            {openMenu === 'user' && (
              <div className="brand-popover brand-user-menu">
                <div className="brand-user-summary">
                  <strong>{user?.username || 'Guest'}</strong>
                  <span>{user?.email}</span>
                </div>
                <button onClick={() => navigate('/user')}>{t('problems.userCenter')}</button>
                <button className="danger" onClick={() => { setOpenMenu(null); logout(); }}>{t('logout')}</button>
              </div>
            )}
          </div>
        </div>
      </div>
    </nav>
  );
}
