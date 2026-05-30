'use client';

import { createContext, useContext, useEffect, useState } from 'react';

type ThemeMode = 'light' | 'dark' | 'system';

interface ThemeContextValue {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  isDark: boolean;
}

const ThemeContext = createContext<ThemeContextValue>({
  mode: 'system',
  setMode: () => {},
  isDark: false,
});

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>('system');
  const [isDark, setIsDark] = useState(false);

  // Load saved preference on mount
  useEffect(() => {
    const saved = localStorage.getItem('themeMode') as ThemeMode | null;
    if (saved === 'light' || saved === 'dark' || saved === 'system') {
      setModeState(saved);
    }
  }, []);

  // Apply dark class to <html> whenever mode or system preference changes
  useEffect(() => {
    const applyTheme = (m: ThemeMode) => {
      let dark: boolean;
      if (m === 'dark') {
        dark = true;
      } else if (m === 'light') {
        dark = false;
      } else {
        dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      }
      setIsDark(dark);
      document.documentElement.classList.toggle('dark', dark);
    };

    applyTheme(mode);

    if (mode === 'system') {
      const mq = window.matchMedia('(prefers-color-scheme: dark)');
      const handler = () => applyTheme('system');
      mq.addEventListener('change', handler);
      return () => mq.removeEventListener('change', handler);
    }
  }, [mode]);

  const setMode = (m: ThemeMode) => {
    setModeState(m);
    localStorage.setItem('themeMode', m);
  };

  return (
    <ThemeContext.Provider value={{ mode, setMode, isDark }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  return useContext(ThemeContext);
}
