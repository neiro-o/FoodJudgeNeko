'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';
import { useDisplayMode } from '@/contexts/DisplayModeContext';

export default function InterfaceSettings({ onOpen }: { onOpen?: () => void }) {
  const { language, setLanguage } = useLanguage();
  const { mode, setMode } = useTheme();
  const { displayMode, setDisplayMode } = useDisplayMode();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const id = useId();
  const zh = language === 'zh';
  const title = zh ? '界面设置' : 'Interface settings';

  useEffect(() => {
    if (!open) return;
    rootRef.current?.querySelector<HTMLInputElement>('input:checked')?.focus();
    const outside = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    document.addEventListener('mousedown', outside);
    document.addEventListener('keydown', escape);
    return () => {
      document.removeEventListener('mousedown', outside);
      document.removeEventListener('keydown', escape);
    };
  }, [open]);

  const option = (group: string, value: string, label: string, checked: boolean, select: () => void) => (
    <label className="interface-setting-option" key={value}>
      <input type="radio" name={`${id}-${group}`} value={value} checked={checked} onChange={select} />
      <span>{label}</span><span className="interface-setting-check" aria-hidden="true">✓</span>
    </label>
  );

  return (
    <div className="interface-settings" ref={rootRef} onBlur={(event) => {
      if (event.relatedTarget && !event.currentTarget.contains(event.relatedTarget as Node)) setOpen(false);
    }}>
      <button ref={triggerRef} className="brand-icon-button interface-settings-trigger" aria-label={title}
        aria-expanded={open} aria-haspopup="dialog" aria-controls={`${id}-panel`}
        onClick={() => { if (!open) onOpen?.(); setOpen(!open); }}>
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 3-.6 2.3-2 .9L4.3 5.5 2 9.5l1.7 1.6v1.8L2 14.5l2.3 4 2.1-.7 2 .9L9 21h6l.6-2.3 2-.9 2.1.7 2.3-4-1.7-1.6v-1.8L22 9.5l-2.3-4-2.1.7-2-.9L15 3Z" /><circle cx="12" cy="12" r="3" /></svg>
      </button>
      {open && (
        <div id={`${id}-panel`} className="brand-popover interface-settings-panel" role="dialog" aria-label={title}>
          <fieldset><legend className="sr-only">{zh ? '语言' : 'Language'}</legend>
            {option('language', 'zh', '中文', language === 'zh', () => setLanguage('zh'))}
            {option('language', 'en', 'English', language === 'en', () => setLanguage('en'))}
          </fieldset>
          <hr />
          <fieldset><legend className="sr-only">{zh ? '主题' : 'Theme'}</legend>
            {option('theme', 'light', zh ? '日间模式' : 'Light mode', mode === 'light', () => setMode('light'))}
            {option('theme', 'dark', zh ? '夜间模式' : 'Dark mode', mode === 'dark', () => setMode('dark'))}
            {option('theme', 'system', zh ? '跟随系统' : 'Follow system', mode === 'system', () => setMode('system'))}
          </fieldset>
          <hr />
          <fieldset><legend className="sr-only">{zh ? '运行模式' : 'Display mode'}</legend>
            {option('display', 'normal', zh ? '普通模式' : 'Normal mode', displayMode === 'normal', () => setDisplayMode('normal'))}
            {option('display', 'energy-saving', zh ? '节能模式' : 'Energy saving', displayMode === 'energy-saving', () => setDisplayMode('energy-saving'))}
          </fieldset>
        </div>
      )}
    </div>
  );
}
