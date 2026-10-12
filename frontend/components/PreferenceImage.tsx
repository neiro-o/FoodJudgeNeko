'use client';

import Image, { ImageProps } from 'next/image';
import { ImgHTMLAttributes } from 'react';
import { useDisplayMode } from '@/contexts/DisplayModeContext';

// Wait for saved preferences before mounting images so energy saving also avoids downloads.
export default function PreferenceImage(props: ImageProps) {
  const { ready, isEnergySaving } = useDisplayMode();
  return ready && !isEnergySaving ? <Image {...props} /> : null;
}

export function PreferenceImg(props: ImgHTMLAttributes<HTMLImageElement>) {
  const { ready, isEnergySaving } = useDisplayMode();
  return ready && !isEnergySaving ? <img {...props} /> : null;
}
