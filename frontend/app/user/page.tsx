'use client';

import { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useRouter } from 'next/navigation';
import { authAPI, invitationAPI, Invitation } from '@/lib/api';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';

type Tab = 'password' | 'points';

export default function UserPage() {
  const { isAuthenticated, loading } = useAuth();
  const { t } = useLanguage();
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<Tab>('password');
  
  // Change password state
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [passwordLoading, setPasswordLoading] = useState(false);

  // Invitations state
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [userPoints, setUserPoints] = useState<number>(0);
  const [inviteCount, setInviteCount] = useState(1);
  const [inviteError, setInviteError] = useState('');
  const [inviteSuccess, setInviteSuccess] = useState('');
  const [inviteLoading, setInviteLoading] = useState(false);
  const [loadingInvitations, setLoadingInvitations] = useState(false);

  useEffect(() => {
    if (!loading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, loading, router]);

  useEffect(() => {
    if (isAuthenticated && activeTab === 'points') {
      loadInvitations();
    }
  }, [isAuthenticated, activeTab]);

  const loadInvitations = async () => {
    setLoadingInvitations(true);
    try {
      const data = await invitationAPI.list();
      setInvitations(data.invitations);
      setUserPoints(data.points);
    } catch (error: any) {
      setInviteError(error.message || 'Failed to load invitations');
    } finally {
      setLoadingInvitations(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');

    if (newPassword !== confirmPassword) {
      setPasswordError(t('user.passwordMismatch'));
      return;
    }

    if (newPassword.length < 6) {
      setPasswordError(t('user.passwordTooShort'));
      return;
    }

    setPasswordLoading(true);
    try {
      await authAPI.changePassword({
        old_password: oldPassword,
        new_password: newPassword,
      });
      setPasswordSuccess(t('user.passwordSuccess'));
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (error: any) {
      setPasswordError(error.response?.data?.error || t('user.passwordError'));
    } finally {
      setPasswordLoading(false);
    }
  };

  const handleGenerateInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    setInviteError('');
    setInviteSuccess('');

    if (inviteCount < 1 || inviteCount > 10) {
      setInviteError(t('user.inviteCountError'));
      return;
    }

    setInviteLoading(true);
    try {
      const response = await invitationAPI.generate({ count: inviteCount });
      setInviteSuccess(t('user.inviteSuccess', { count: response.count }));
      setInviteCount(1);
      loadInvitations();
    } catch (error: any) {
      setInviteError(error.response?.data?.error || t('user.inviteError'));
    } finally {
      setInviteLoading(false);
    }
  };

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
      <PageTitle titleKey="pageTitle.user" />
      <div className="min-h-screen bg-gray-50">
        <Navbar title={t('user.title')} />

        <main className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="bg-white rounded-lg shadow-sm">
          {/* Tabs */}
          <div className="border-b border-gray-200">
            <nav className="flex -mb-px">
              <button
                onClick={() => setActiveTab('password')}
                className={`py-4 px-6 text-sm font-medium border-b-2 transition ${
                  activeTab === 'password'
                    ? 'border-indigo-500 text-indigo-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                {t('user.changePassword')}
              </button>
              <button
                onClick={() => setActiveTab('points')}
                className={`py-4 px-6 text-sm font-medium border-b-2 transition ${
                  activeTab === 'points'
                    ? 'border-indigo-500 text-indigo-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                {t('user.pointsInvitations')}
              </button>
            </nav>
          </div>

          {/* Tab Content */}
          <div className="p-6">
            {/* Change Password Tab */}
            {activeTab === 'password' && (
              <div className="max-w-md">
                <h2 className="text-2xl font-bold text-gray-900 mb-6">{t('user.changePassword')}</h2>
                
                {passwordError && (
                  <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                    <p className="text-sm text-red-600">{passwordError}</p>
                  </div>
                )}

                {passwordSuccess && (
                  <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-lg">
                    <p className="text-sm text-green-600">{passwordSuccess}</p>
                  </div>
                )}

                <form onSubmit={handleChangePassword} className="space-y-4">
                  <div>
                    <label htmlFor="oldPassword" className="block text-sm font-medium text-gray-700 mb-2">
                      {t('user.oldPassword')}
                    </label>
                    <input
                      id="oldPassword"
                      type="password"
                      value={oldPassword}
                      onChange={(e) => setOldPassword(e.target.value)}
                      required
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
                    />
                  </div>

                  <div>
                    <label htmlFor="newPassword" className="block text-sm font-medium text-gray-700 mb-2">
                      {t('user.newPassword')}
                    </label>
                    <input
                      id="newPassword"
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      required
                      minLength={6}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
                    />
                  </div>

                  <div>
                    <label htmlFor="confirmPassword" className="block text-sm font-medium text-gray-700 mb-2">
                      {t('user.confirmNewPassword')}
                    </label>
                    <input
                      id="confirmPassword"
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      required
                      minLength={6}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
                    />
                  </div>

                  <button
                    type="submit"
                    disabled={passwordLoading}
                    className="w-full bg-indigo-600 text-white py-2 rounded-lg font-medium hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition"
                  >
                    {passwordLoading ? t('user.submittingPassword') : t('user.submitPassword')}
                  </button>
                </form>
              </div>
            )}

            {/* Points & Invitations Tab */}
            {activeTab === 'points' && (
              <div>
                <h2 className="text-2xl font-bold text-gray-900 mb-6">{t('user.pointsInvitations')}</h2>
                
                {/* Points Display */}
                <div className="mb-8 p-4 bg-indigo-50 rounded-lg border border-indigo-200">
                  <p className="text-sm text-indigo-700 mb-1">{t('user.yourPoints')}</p>
                  <p className="text-3xl font-bold text-indigo-900">{userPoints}</p>
                </div>

                {/* Generate Invitation Code */}
                <div className="mb-8">
                  <h3 className="text-lg font-semibold text-gray-900 mb-4">{t('user.generateInvite')}</h3>
                  
                  {inviteError && (
                    <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                      <p className="text-sm text-red-600">{inviteError}</p>
                    </div>
                  )}

                  {inviteSuccess && (
                    <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-lg">
                      <p className="text-sm text-green-600">{inviteSuccess}</p>
                    </div>
                  )}

                  <form onSubmit={handleGenerateInvite} className="space-y-4">
                    <div>
                      <label htmlFor="inviteCount" className="block text-sm font-medium text-gray-700 mb-2">
                        {t('user.inviteCount')}
                      </label>
                      <input
                        id="inviteCount"
                        type="number"
                        min="1"
                        max="10"
                        value={inviteCount}
                        onChange={(e) => setInviteCount(parseInt(e.target.value) || 1)}
                        required
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
                      />
                    </div>

                    <button
                      type="submit"
                      disabled={inviteLoading}
                      className="bg-indigo-600 text-white py-2 px-6 rounded-lg font-medium hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition"
                    >
                      {inviteLoading ? t('user.generating') : t('user.generateCodes')}
                    </button>
                  </form>
                </div>

                {/* Invitations List */}
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 mb-4">{t('user.myInvitations')}</h3>
                  
                  {loadingInvitations ? (
                    <div className="text-center py-8 text-gray-600">{t('user.loadingInvitations')}</div>
                  ) : !invitations || invitations.length === 0 ? (
                    <div className="text-center py-8 text-gray-500">{t('user.noInvitations')}</div>
                  ) : (
                    <div className="space-y-2">
                      {invitations.map((invitation) => (
                        <div
                          key={invitation.code}
                          className={`p-4 rounded-lg border ${
                            invitation.used_by !== null
                              ? 'bg-gray-50 border-gray-200'
                              : 'bg-green-50 border-green-200'
                          }`}
                        >
                          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
                            <div>
                              <p className="font-mono text-sm font-medium text-gray-900">
                                {invitation.code}
                              </p>
                              <p className="text-xs text-gray-500 mt-1">
                                {t('user.created')}: {new Date(invitation.created_at).toLocaleString()}
                              </p>
                            </div>
                            <div>
                              <span
                                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                                  invitation.used_by !== null
                                    ? 'bg-gray-200 text-gray-800'
                                    : 'bg-green-200 text-green-800'
                                }`}
                              >
                                {invitation.used_by !== null ? t('user.used') : t('user.available')}
                              </span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
    </>
  );
}

