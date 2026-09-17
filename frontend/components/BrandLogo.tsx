'use client';

import Link from 'next/link';

export default function BrandLogo({ compact = false }: { compact?: boolean }) {
  return (
    <Link href="/problems" className="brand-logo" aria-label="搜题首页">
      <span className="brand-logo-mark" aria-hidden="true">
        <svg viewBox="0 0 48 48" role="img">
          <path d="M7 10.5 22.5 16v23L7 32.5zM41 10.5 25.5 16v23L41 32.5z" fill="currentColor" />
          <path d="M14 17.5v10.4M34 17.5v10.4M14 18l6 2.2M34 18l-6 2.2" fill="none" stroke="#111827" strokeWidth="2.6" strokeLinecap="round" />
        </svg>
      </span>
      <span className="brand-logo-copy">
        <strong>搜题</strong>
        {!compact && <small>美团评审团 · 题目答案搜索</small>}
      </span>
    </Link>
  );
}
