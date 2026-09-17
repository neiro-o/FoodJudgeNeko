'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter } from 'next/navigation';
import { authAPI } from '@/lib/api';
import Link from 'next/link';
import Image from 'next/image';
import BrandLogo from '@/components/BrandLogo';
import PageTitle from '@/components/PageTitle';

export default function RegisterPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [inviteCode, setInviteCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { isAuthenticated } = useAuth();
  const { t } = useLanguage();
  const router = useRouter();

  useEffect(() => {
    if (isAuthenticated) router.push('/problems');
  }, [isAuthenticated, router]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    if (password !== confirmPassword) return setError(t('register.passwordMismatch'));
    if (password.length < 6) return setError(t('register.passwordTooShort'));
    setLoading(true);
    try {
      await authAPI.register({ username, email, password, invite_code: inviteCode });
      router.push('/login?registered=true');
    } catch (registerError: any) {
      setError(registerError.message || t('register.error'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <PageTitle titleKey="pageTitle.register" />
      <div className="brand-page auth-page">
        <section className="auth-visual">
          <BrandLogo />
          <div className="auth-kicker">一起把题库，<br /><span>变得更可靠。</span></div>
          <Image className="auth-mascot" src="/brand/kangaroo-reader.png?v=2" alt="读题袋鼠" width={1254} height={1254} priority unoptimized />
        </section>
        <section className="auth-panel">
          <div className="brand-card auth-card my-8">
            <p className="search-eyebrow">CREATE ACCOUNT</p>
            <h1>{t('register.title')}</h1>
            <p className="mt-2 mb-5 text-gray-500">{t('register.subtitle')}</p>
            <div className="search-alert warning">{t('register.notice')}</div>
            {error && <div className="search-alert error">{error}</div>}
            <form onSubmit={handleSubmit} className="space-y-4">
              <div><label htmlFor="inviteCode">{t('register.inviteCode')}</label><input className="brand-form-input" id="inviteCode" value={inviteCode} onChange={(event) => setInviteCode(event.target.value)} placeholder={t('register.inviteCodePlaceholder')} required /></div>
              <div><label htmlFor="username">{t('register.username')}</label><input className="brand-form-input" id="username" value={username} onChange={(event) => setUsername(event.target.value)} placeholder={t('register.usernamePlaceholder')} required /></div>
              <div><label htmlFor="email">{t('register.email')}</label><input className="brand-form-input" id="email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder={t('register.emailPlaceholder')} required /></div>
              <div><label htmlFor="password">{t('register.password')}</label><input className="brand-form-input" id="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder={t('register.passwordPlaceholder')} required minLength={6} /></div>
              <div><label htmlFor="confirmPassword">{t('register.confirmPassword')}</label><input className="brand-form-input" id="confirmPassword" type="password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} placeholder={t('register.confirmPasswordPlaceholder')} required minLength={6} /></div>
              <button className="brand-primary-button w-full py-3.5" type="submit" disabled={loading}>{loading ? t('register.submitting') : t('register.submit')}</button>
            </form>
            <p className="mt-5 text-center text-sm text-gray-500">{t('register.hasAccount')} <Link className="font-bold text-amber-600" href="/login">{t('register.signIn')}</Link></p>
          </div>
        </section>
      </div>
    </>
  );
}
