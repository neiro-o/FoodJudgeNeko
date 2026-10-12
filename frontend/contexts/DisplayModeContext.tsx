'use client';

import { createContext, useContext, useEffect, useState } from 'react';

export type DisplayMode = 'normal' | 'energy-saving';
const DisplayModeContext = createContext({
  displayMode: 'normal' as DisplayMode,
  setDisplayMode: (_mode: DisplayMode) => {},
  isEnergySaving: false,
  ready: false,
});

export function DisplayModeProvider({ children }: { children: React.ReactNode }) {
  const [displayMode, setDisplayModeState] = useState<DisplayMode>('normal');
  const [ready, setReady] = useState(false);

  useEffect(() => {
    try {
      if (localStorage.getItem('displayMode') === 'energy-saving') {
        setDisplayModeState('energy-saving');
      }
    } catch { /* Preferences still work when browser storage is unavailable. */ }
    setReady(true);
    const sync = (event: StorageEvent) => {
      if (event.key === 'displayMode' || event.key === null) {
        setDisplayModeState(event.newValue === 'energy-saving' ? 'energy-saving' : 'normal');
      }
    };
    window.addEventListener('storage', sync);
    return () => window.removeEventListener('storage', sync);
  }, []);

  useEffect(() => {
    if (ready) document.documentElement.dataset.displayMode = displayMode;
  }, [displayMode, ready]);

  const setDisplayMode = (mode: DisplayMode) => {
    document.documentElement.dataset.displayMode = mode;
    setDisplayModeState(mode);
    try { localStorage.setItem('displayMode', mode); } catch { /* Keep the current selection. */ }
  };

  return (
    <DisplayModeContext.Provider value={{ displayMode, setDisplayMode, isEnergySaving: displayMode === 'energy-saving', ready }}>
      {children}
    </DisplayModeContext.Provider>
  );
}

export const useDisplayMode = () => useContext(DisplayModeContext);
