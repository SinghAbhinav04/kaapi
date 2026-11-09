import type { ImageLoaderProps } from 'next/image';

/**
 * Widths that scripts/build-images.py writes next to every photo (name-480.avif, …).
 * Keep in step with VARIANTS there.
 */
const VARIANT_WIDTHS = [480, 960, 1600] as const;

/**
 * Picks the smallest pre-built copy at least as wide as the slot needs; beyond the largest,
 * the original. The copies are made at build time, not by Vercel's optimizer, which passes
 * AVIF sources through untouched (phones were downloading 2400 px photos) and has a monthly
 * quota. Quality is baked into the files, so `quality` is ignored.
 */
export default function imageLoader({ src, width }: ImageLoaderProps): string {
  if (!src.startsWith('/images/') || !src.endsWith('.avif')) return src;
  const fit = VARIANT_WIDTHS.find((w) => w >= width);
  // The width stays in the URL so each srcset entry is distinct (static hosting ignores it).
  return fit ? src.replace(/\.avif$/, `-${fit}.avif`) : `${src}?w=${width}`;
}
