'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import InterfaceSettings from '@/components/InterfaceSettings';
import BrandLogo from '@/components/BrandLogo';
import AccountAvatar from '@/components/AccountAvatar';

interface NavbarProps {
  title: string;
  showBackButton?: boolean;
  backHref?: string;
}

export default function Navbar({ title, showBackButton = false, backHref = '/problems' }: NavbarProps) {
  const { logout, user } = useAuth();
  const { t } = useLanguage();
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
          <InterfaceSettings onOpen={() => setOpenMenu(null)} />
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
