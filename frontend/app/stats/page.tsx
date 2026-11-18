'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from '@/lib/navigation';
import Navbar from '@/components/Navbar';
import PageTitle from '@/components/PageTitle';
import StatsWordCloud from '@/components/StatsWordCloud';
import { useAuth } from '@/contexts/AuthContext';
import { useLanguage } from '@/contexts/LanguageContext';
import { useDisplayMode } from '@/contexts/DisplayModeContext';
import { getLoginUrl } from '@/lib/auth-redirect';
import { ApiError, statsAPI, WordCloudComment, WordCloudCommentsResponse, WordCloudRange, WordCloudResponse, WordCloudWord } from '@/lib/api';
import styles from './stats.module.css';

const copy = {
  zh: {
    title: '评论词云', subtitle: '从评论里，看看大家最近在讨论什么。', day: '近 24 小时', week: '近 7 天',
    comments: '条有效评论', words: '个热词', updated: '更新于', loading: '正在整理评论与热词…', empty: '这个时间范围内还没有可展示的热词。',
    error: '词云加载失败，请重试。', detailError: '评论加载失败，请重试。', retry: '重新加载',
    hint: '悬停或聚焦词语预览评论，点击查看全部。', count: '条评论提及', all: '查看全部评论',
    list: '词频列表', close: '关闭', prev: '上一页', next: '下一页', page: '页', anonymous: '匿名用户', unknown: '用户',
    note: '按提及评论数统计，同一条评论重复出现的词只计一次。已排除恶意用户名单、常用停用词及无效文本；约每 10 分钟更新。',
    expired: '统计快照已过期，已重新加载词云，请再次选择词语。', from: '统计范围', likes: '赞',
  },
  en: {
    title: 'Comment word cloud', subtitle: 'See what people have been talking about lately.', day: 'Last 24 hours', week: 'Last 7 days',
    comments: 'valid comments', words: 'top words', updated: 'Updated', loading: 'Collecting comments and topics…', empty: 'No topics to display in this time range.',
    error: 'Could not load the word cloud. Please retry.', detailError: 'Could not load comments. Please retry.', retry: 'Reload',
    hint: 'Hover or focus a word to preview comments. Select it to see all.', count: 'comments mention this', all: 'View all comments',
    list: 'Word frequency list', close: 'Close', prev: 'Previous', next: 'Next', page: 'page', anonymous: 'Anonymous', unknown: 'User',
    note: 'Ranked by comments mentioning each word, counted once per comment. Malicious users, stop words and invalid text are excluded. Updates about every 10 minutes.',
    expired: 'The snapshot expired. The cloud has refreshed; please select a word again.', from: 'Time range', likes: 'likes',
  },
};

export default function StatsPage() {
  const router = useRouter();
  const { isAuthenticated, loading: authLoading } = useAuth();
  const { language, t } = useLanguage();
  const { isEnergySaving } = useDisplayMode();
  const c = copy[language];
  const [range, setRange] = useState<WordCloudRange>('24h');
  const [refresh, setRefresh] = useState(0);
  const [data, setData] = useState<WordCloudResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [notice, setNotice] = useState(false);
  const [preview, setPreview] = useState<{ word: WordCloudWord; left: number; top: number } | null>(null);
  const [selected, setSelected] = useState<WordCloudWord | null>(null);
  const [page, setPage] = useState(1);
  const [details, setDetails] = useState<WordCloudCommentsResponse | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState(false);
  const [detailRetry, setDetailRetry] = useState(0);
  const wrapper = useRef<HTMLDivElement>(null);
  const panel = useRef<HTMLElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout>>();
  const previewRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!authLoading && !isAuthenticated) router.replace(getLoginUrl());
  }, [authLoading, isAuthenticated, router]);

  useEffect(() => {
    if (!isAuthenticated) return;
    const controller = new AbortController();
    setLoading(true); setError(false); setData(null); setSelected(null); setPreview(null); setDetails(null); setPage(1);
    statsAPI.wordCloud(range, controller.signal).then(response => {
      if (!controller.signal.aborted) setData(response);
    }).catch(() => { if (!controller.signal.aborted) setError(true); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [isAuthenticated, range, refresh]);

  useEffect(() => {
    if (!isAuthenticated) return;
    const timer = setInterval(() => setRefresh(value => value + 1), 10 * 60 * 1000);
    return () => clearInterval(timer);
  }, [isAuthenticated]);

  useEffect(() => {
    if (!selected || !data) return;
    const controller = new AbortController();
    setDetailLoading(true); setDetailError(false); setDetails(null);
    statsAPI.comments(range, data.snapshot, selected.text, page, controller.signal).then(response => {
      if (!controller.signal.aborted) setDetails(response);
    }).catch(error => {
      if (controller.signal.aborted) return;
      if (error instanceof ApiError && error.code === 410) { setNotice(true); setRefresh(value => value + 1); }
      else setDetailError(true);
    }).finally(() => { if (!controller.signal.aborted) setDetailLoading(false); });
    return () => controller.abort();
  }, [selected, data, page, range, detailRetry]);

  useEffect(() => {
    if (selected) {
      panel.current?.focus({ preventScroll: true });
      panel.current?.scrollIntoView({ behavior: isEnergySaving ? 'instant' : 'smooth', block: 'start' });
    }
  }, [selected, page, isEnergySaving]);

  useEffect(() => {
    const clearPreview = () => setPreview(null);
    window.addEventListener('resize', clearPreview);
    return () => { window.removeEventListener('resize', clearPreview); clearTimeout(closeTimer.current); };
  }, []);
  const holdPreview = () => clearTimeout(closeTimer.current);
  const leavePreview = () => {
    holdPreview();
    closeTimer.current = setTimeout(() => {
      if (!previewRef.current?.contains(document.activeElement)) setPreview(null);
    }, 180);
  };
  const showPreview = (word: WordCloudWord, bounds: DOMRect) => {
    holdPreview();
    const rect = wrapper.current!.getBoundingClientRect();
    const width = Math.min(320, rect.width - 24);
    const height = Math.min(400, window.innerHeight - 100);
    const preferredTop = bounds.bottom + height + 8 > window.innerHeight ? bounds.top - height - 8 : bounds.bottom + 8;
    const top = Math.max(80, Math.min(preferredTop, window.innerHeight - height - 12)) - rect.top;
    setPreview({ word, left: Math.max(12, Math.min(bounds.left - rect.left, rect.width - width - 12)), top });
  };
  const select = (word: WordCloudWord) => { holdPreview(); setPreview(null); setSelected(word); setPage(1); setNotice(false); };
  const changeRange = (value: WordCloudRange) => {
    if (value === range) return;
    setData(null); setSelected(null); setPreview(null); setDetails(null); setNotice(false); setRange(value);
  };
  const date = (unix: number) => new Date(unix * 1000).toLocaleString(language === 'zh' ? 'zh-CN' : 'en-GB', { hour12: false });
  const renderComment = (comment: WordCloudComment) => (
    <article key={comment.id} className={styles.comment}>
      <div className={styles.commentMeta}>
        <span>{comment.isAnonymous ? c.anonymous : comment.name || c.unknown}</span>
        <time dateTime={new Date(comment.createTime * 1000).toISOString()}>{date(comment.createTime)}</time>
        <span>{comment.likes} {c.likes}</span>
      </div>
      <p>{comment.content}</p>
    </article>
  );

  return (
    <div className={styles.page}>
      <PageTitle titleKey="pageTitle.stats" />
      <Navbar title={t('nav.stats')} />
      <main className={styles.main}>
        <header className={styles.header}>
          <div><div className={styles.eyebrow}>MTV2 / {t('nav.stats')}</div><h1>{c.title}</h1><p className={styles.muted}>{c.subtitle}</p></div>
          <div className={styles.tabs} role="group" aria-label={c.from}>
            <button aria-pressed={range === '24h'} onClick={() => changeRange('24h')}>{c.day}</button>
            <button aria-pressed={range === '7d'} onClick={() => changeRange('7d')}>{c.week}</button>
          </div>
        </header>
        {notice && <p className={styles.note} role="status">{c.expired}</p>}
        <section className={styles.card} aria-label={c.title} aria-busy={loading}>
          {authLoading || loading ? <div className={styles.message} role="status">{c.loading}</div>
            : error ? <div className={styles.message} role="alert">{c.error}<button className={styles.button} onClick={() => setRefresh(value => value + 1)}>{c.retry}</button></div>
              : data && <>
                <div className={styles.meta}>
                  <div><strong>{data.commentCount.toLocaleString()}</strong><span className={styles.muted}>{c.comments}</span></div>
                  <div><strong>{data.words.length}</strong><span className={styles.muted}>{c.words}</span></div>
                  <div className={`${styles.muted} ${styles.updated}`}>{c.updated} {date(data.generatedAt)}</div>
                  <div className={styles.muted} style={{ width: '100%' }}>{c.from}: {date(data.from)} — {date(data.to)}</div>
                </div>
                {!data.words.length ? <div className={styles.message}>{c.empty}</div> : <>
                  <div className={styles.cloud} ref={wrapper}>
                    <StatsWordCloud words={data.words} onPreview={showPreview} onLeave={leavePreview} onSelect={select} label={c.title} loadingLabel={c.loading} />
                    {preview && <div ref={previewRef} id="stats-word-preview" className={styles.preview} style={{ left: preview.left, top: preview.top }}
                      onPointerEnter={holdPreview} onPointerLeave={leavePreview} onFocus={holdPreview} onBlur={leavePreview}
                      onKeyDown={event => { if (event.key === 'Escape') setPreview(null); }}>
                      <header><strong>{preview.word.text}</strong><button aria-label={c.close} onClick={() => setPreview(null)}>×</button></header>
                      <div className={styles.muted}>{preview.word.count} {c.count}</div>
                      {preview.word.preview.map(renderComment)}
                      <button className={styles.button} onClick={() => select(preview.word)}>{c.all}</button>
                    </div>}
                  </div>
                  <p className={styles.hint}>{c.hint}</p>
                  <details className={styles.list}><summary>{c.list} ({data.words.length})</summary><div className={styles.listWords}>
                    {data.words.map(word => <button key={word.text} onClick={() => select(word)}>{word.text}<span>{word.count}</span></button>)}
                  </div></details>
                </>}
              </>}
        </section>
        <p className={`${styles.muted} ${styles.note}`}>{c.note}</p>
        {selected && <section ref={panel} tabIndex={-1} className={`${styles.card} ${styles.panel}`} aria-label={`${selected.text} ${c.all}`}
          onKeyDown={event => { if (event.key === 'Escape') setSelected(null); }} aria-busy={detailLoading}>
          <header className={styles.panelHeader}><h2>{selected.text} <span className={styles.muted}>· {selected.count} {c.count}</span></h2>
            <button className={styles.button} onClick={() => setSelected(null)}>{c.close}</button></header>
          {detailLoading ? <div className={styles.message} role="status">{c.loading}</div>
            : detailError ? <div className={styles.message} role="alert">{c.detailError}<button className={styles.button} onClick={() => setDetailRetry(value => value + 1)}>{c.retry}</button></div>
              : details && <><div>{details.comments.map(renderComment)}</div>
                <div className={styles.pagination}>
                  <button className={styles.button} disabled={page <= 1} onClick={() => setPage(value => value - 1)}>{c.prev}</button>
                  <span className={styles.muted}>{page} / {details.totalPages}</span>
                  <button className={styles.button} disabled={page >= details.totalPages} onClick={() => setPage(value => value + 1)}>{c.next}</button>
                </div></>}
        </section>}
      </main>
    </div>
  );
}
