'use client';

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { SearchResult } from '@/lib/api';
import { ColumnConfig, DEFAULT_COLUMNS } from '@/components/ColumnCustomizer';

interface ProblemsState {
  // Upload states
  uploadMode: 'single' | 'multiple';
  singleUrl: string;
  singleParsed: { userId: string; taskId: string } | null;
  multipleUrls: string;
  multipleParsed: {
    pairs: Array<{ userId: string; taskId: string }>;
    userIdCount: number;
    taskIdCount: number;
    isValid: boolean;
  };
  uploadError: string;
  uploadSuccess: string;
  uploadLoading: boolean;

  // Search states
  searchKeyword: string;
  currentSearchKeyword: string;
  searchResults: SearchResult[];
  searchTotal: number;
  searchLoading: boolean;
  searchError: string;

  // Count states
  counts: { elasticsearch: number; mongodb: number; redis: number } | null;
  countsLoading: boolean;

  // Column customization states
  columns: ColumnConfig[];
  isCustomizerOpen: boolean;
}

interface ProblemsStateContextType {
  state: ProblemsState;
  setState: React.Dispatch<React.SetStateAction<ProblemsState>>;
  updateState: (updates: Partial<ProblemsState>) => void;
}

const defaultState: ProblemsState = {
  uploadMode: 'single',
  singleUrl: '',
  singleParsed: null,
  multipleUrls: '',
  multipleParsed: { pairs: [], userIdCount: 0, taskIdCount: 0, isValid: false },
  uploadError: '',
  uploadSuccess: '',
  uploadLoading: false,
  searchKeyword: '',
  currentSearchKeyword: '',
  searchResults: [],
  searchTotal: 0,
  searchLoading: false,
  searchError: '',
  counts: null,
  countsLoading: false,
  columns: DEFAULT_COLUMNS,
  isCustomizerOpen: false,
};

const ProblemsStateContext = createContext<ProblemsStateContextType | undefined>(undefined);

export function ProblemsStateProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ProblemsState>(() => {
    // Try to restore from localStorage on mount
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('problemsPageState');
      if (saved) {
        try {
          const parsed = JSON.parse(saved);
          // Restore columns from localStorage if available
          const savedColumns = localStorage.getItem('problemTableColumns');
          if (savedColumns) {
            try {
              parsed.columns = JSON.parse(savedColumns);
            } catch (e) {
              // Keep default columns if parsing fails
            }
          }
          return { ...defaultState, ...parsed, columns: parsed.columns || defaultState.columns };
        } catch (e) {
          console.error('Failed to restore problems page state', e);
        }
      }
    }
    return defaultState;
  });

  // Save to localStorage whenever state changes (debounced)
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const timeoutId = setTimeout(() => {
        const stateToSave = {
          ...state,
          // Don't save loading states or temporary UI states
          uploadLoading: false,
          searchLoading: false,
          countsLoading: false,
          isCustomizerOpen: false,
          uploadError: '',
          uploadSuccess: '',
          searchError: '',
        };
        localStorage.setItem('problemsPageState', JSON.stringify(stateToSave));
      }, 500);
      return () => clearTimeout(timeoutId);
    }
  }, [state]);

  const updateState = (updates: Partial<ProblemsState>) => {
    setState((prev) => ({ ...prev, ...updates }));
  };

  return (
    <ProblemsStateContext.Provider value={{ state, setState, updateState }}>
      {children}
    </ProblemsStateContext.Provider>
  );
}

export function useProblemsState() {
  const context = useContext(ProblemsStateContext);
  if (context === undefined) {
    throw new Error('useProblemsState must be used within a ProblemsStateProvider');
  }
  return context;
}
