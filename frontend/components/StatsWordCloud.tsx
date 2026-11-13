'use client';

import { useEffect, useRef, useState } from 'react';
import cloud from 'd3-cloud';
import type { WordCloudWord } from '@/lib/api';
import { useTheme } from '@/contexts/ThemeContext';
import styles from '@/app/stats/stats.module.css';

interface PlacedWord extends WordCloudWord { x?: number; y?: number; size?: number }
interface Props {
  words: WordCloudWord[];
  onPreview: (word: WordCloudWord, bounds: DOMRect) => void;
  onLeave: () => void;
  onSelect: (word: WordCloudWord) => void;
  label: string;
  loadingLabel: string;
}

export default function StatsWordCloud({ words, onPreview, onLeave, onSelect, label, loadingLabel }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 480 });
  const [placed, setPlaced] = useState<PlacedWord[]>([]);
  const [font, setFont] = useState('sans-serif');
  const { isDark } = useTheme();

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    const observer = new ResizeObserver(([entry]) => {
      const width = Math.floor(entry.contentRect.width);
      const height = Math.floor(entry.contentRect.height);
      setSize(previous => previous.width === width && previous.height === height ? previous : { width, height });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!size.width || !words.length) { setPlaced([]); return; }
    setPlaced([]);
    let active = true;
    let layout: ReturnType<typeof cloud<PlacedWord>> | undefined;
    const arrange = async () => {
      const family = getComputedStyle(host.current!).fontFamily;
      await document.fonts.load(`500 20px ${family}`);
      await document.fonts.ready;
      if (!active) return;
      setFont(family);
      const max = Math.max(...words.map(w => w.count));
      const min = Math.min(...words.map(w => w.count));
      let attempt = 0;
      const run = () => {
        let seed = 42;
        layout = cloud<PlacedWord>()
          .size([size.width, size.height]).padding(size.width < 640 ? 2 : 4).rotate(0)
          .font(family).fontWeight(500).timeInterval(12)
          .random(() => { seed = (seed * 1664525 + 1013904223) >>> 0; return seed / 4294967296; })
          .words(words.map(w => ({ ...w, size: (14 + (max === min ? 12 : 46 * Math.sqrt((w.count - min) / (max - min)))) * Math.pow(.85, attempt) })))
          .fontSize(w => w.size!)
          .on('end', result => {
            if (!active) return;
            if (result.length < words.length && attempt < 8) { attempt++; run(); }
            else setPlaced(result);
          });
        layout.start();
      };
      run();
    };
    void arrange();
    return () => { active = false; layout?.stop(); };
  }, [words, size]);

  const palette = isDark
    ? ['#ebc59d', '#aebed5', '#b6c8b1', '#c8b2cf', '#d9ad9e', '#9fc9c9']
    : ['#91633b', '#576f90', '#607e60', '#85678f', '#a36552', '#467e80'];

  return (
    <div className={styles.canvas} ref={host}>
      {!placed.length && <div className={styles.message} role="status">{loadingLabel}</div>}
      <svg viewBox={`0 0 ${size.width || 800} ${size.height}`} role="group" aria-label={label}>
        <g transform={`translate(${size.width / 2},${size.height / 2})`}>
          {placed.map((word, index) => (
            <text key={word.text} className={styles.word} role="button" tabIndex={0}
              aria-label={`${word.text}: ${word.count}`} aria-describedby="stats-word-preview"
              x={word.x} y={word.y} textAnchor="middle" fontFamily={font} fontSize={word.size} fontWeight={500}
              fill={palette[index % palette.length]}
              onPointerEnter={event => { if (event.pointerType !== 'touch') onPreview(word, event.currentTarget.getBoundingClientRect()); }}
              onPointerLeave={onLeave}
              onFocus={event => onPreview(word, event.currentTarget.getBoundingClientRect())} onBlur={onLeave}
              onClick={() => onSelect(word)}
              onKeyDown={event => {
                if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect(word); }
                if (event.key === 'Escape') onLeave();
              }}
            >{word.text}</text>
          ))}
        </g>
      </svg>
    </div>
  );
}
