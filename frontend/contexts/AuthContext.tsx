'use client';

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { useRouter } from '@/lib/navigation';
import { authAPI, restoreAutomaticLogin } from '@/lib/api';
import { AUTH_CHANGED_EVENT, clearAuth, getStoredAuth, LoginOptions, startLogin, tokenIsExpired } from '@/lib/auth-storage';

interface User {
  id: string;
  username: string;
  email: string;
  points: number;
  is_admin: boolean;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (username: string, password: string, options: LoginOptions) => Promise<void>;
  logout: () => Promise<void>;
  isAuthenticated: boolean;
  loading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const previewMode = process.env.NEXT_PUBLIC_UI_PREVIEW === '1';
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    if (previewMode) {
      setToken('ui-preview');
      setUser({ id: 'preview', username: '设计预览', email: 'preview@local', points: 360, is_admin: false });
      setLoading(false);
      return;
    }
    let active = true;
    const sync = () => {
      const stored = getStoredAuth();
      setToken(stored?.token || null);
      setUser(stored?.user || null);
    };
    const restore = async () => {
      try {
        const stored = getStoredAuth();
        if (!stored || tokenIsExpired(stored.token)) await restoreAutomaticLogin();
      } catch (error) {
        console.error('Session restore failed:', error);
      } finally {
        if (active) { sync(); setLoading(false); }
      }
    };
    void restore();
    window.addEventListener(AUTH_CHANGED_EVENT, sync);
    window.addEventListener('storage', sync);
    window.addEventListener('focus', restore);
    return () => {
      active = false;
      window.removeEventListener(AUTH_CHANGED_EVENT, sync);
      window.removeEventListener('storage', sync);
      window.removeEventListener('focus', restore);
    };
  }, [previewMode]);

  const login = async (username: string, password: string, options: LoginOptions) => {
    try {
      const response = await authAPI.login({ username, password });
      startLogin(response, { username, password }, options);
      setToken(response.token);
      setUser(response.user);
    } catch (error: any) {
      throw new Error(error.message || 'Login failed');
    }
  };

  const logout = async () => {
    try {
      await authAPI.logout();
    } catch (error) {
      // Continue with logout even if API call fails
      console.error('Logout error:', error);
    } finally {
      setToken(null);
      setUser(null);
      
      if (typeof window !== 'undefined') {
        clearAuth();
      }
      
      router.push('/login');
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        logout,
        isAuthenticated: !!token && !!user,
        loading,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
