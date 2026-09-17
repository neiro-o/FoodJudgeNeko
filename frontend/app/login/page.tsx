'use client';

import { FormEvent, Suspense, useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import Image from 'next/image';
import BrandLogo from '@/components/BrandLogo';
import PageTitle from '@/components/PageTitle';

function LoginForm() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [loading, setLoading] = useState(false);
  const { login, isAuthenticated } = useAuth();
  const { t } = useLanguage();
  const router = useRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    if (searchParams.get('registered') === 'true') setSuccess(t('login.success'));
    if (searchParams.get('expired') === 'true') setError(t('login.expired'));
  }, [searchParams, t]);

  useEffect(() => {
    if (isAuthenticated) router.push('/problems');
  }, [isAuthenticated, router]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username, password);
    } catch (loginError: any) {
      setError(loginError.message || t('login.error'));
    } finally {
      setLoading(false);
    }
  };

  if (isAuthenticated) return null;

  return (
    <div className="brand-page auth-page">
      <section className="auth-visual">
        <BrandLogo />
        <div className="auth-kicker">认真搜题，<br /><span>少走一点弯路。</span></div>
        <Image className="auth-mascot" src="/brand/kangaroo-reader.png?v=2" alt="读题袋鼠" width={1254} height={1254} priority unoptimized />
      </section>
      <section className="auth-panel">
        <div className="brand-card auth-card">
          <p className="search-eyebrow">WELCOME BACK</p>
          <h1>{t('login.title')}</h1>
          <p className="mt-2 mb-8 text-gray-500">{t('login.subtitle')}</p>
          {error && <div className="search-alert error">{error}</div>}
          {success && <div className="search-alert success">{success}</div>}
          <form onSubmit={handleSubmit} className="space-y-5">
            <div><label htmlFor="username">{t('login.username')}</label><input className="brand-form-input" id="username" value={username} onChange={(event) => setUsername(event.target.value)} placeholder={t('login.usernamePlaceholder')} required /></div>
            <div><label htmlFor="password">{t('login.password')}</label><input className="brand-form-input" id="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder={t('login.passwordPlaceholder')} required /></div>
            <button className="brand-primary-button w-full py-3.5" type="submit" disabled={loading}>{loading ? t('login.submitting') : t('login.submit')}</button>
          </form>
          <p className="mt-6 text-center text-sm text-gray-500">{t('login.noAccount')} <Link className="font-bold text-amber-600" href="/register">{t('login.signUp')}</Link></p>
        </div>
      </section>
    </div>
  );
}

export default function LoginPage() {
  return <><PageTitle titleKey="pageTitle.login" /><Suspense fallback={<div className="brand-page min-h-screen" />}><LoginForm /></Suspense></>;
}
