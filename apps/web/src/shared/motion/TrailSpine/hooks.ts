'use client';

import { useEffect, useRef, useState } from 'react';

import { useReducedMotion } from '@/shared/hooks/useMediaQuery';

import { TRAIL_ANCHOR_Y, TRAIL_MIN_WIDTH, TRAIL_RESIZE_DEBOUNCE_MS, TRAIL_SAMPLES, TRAIL_SIDE_X } from './const';
import { catmullRomPath, lowerIndex } from './geometry';
import type { TrailGeometry, TrailSide } from './types';

/**
 * Measures the [data-trail] sections inside the trail's parent and returns the path. Scroll
 * then reveals the line and moves the dot by writing transforms directly: no state updates
 * and no repaints while scrolling. Only a resize re-measures.
 */
export function useTrailSpine() {
  const revealRef = useRef<HTMLDivElement>(null);
  const innerRef = useRef<HTMLDivElement>(null);
  const pathRef = useRef<SVGPathElement>(null);
  const dotRef = useRef<HTMLDivElement>(null);
  const [geometry, setGeometry] = useState<TrailGeometry | null>(null);
  const reduced = useReducedMotion();

  // Measure on mount and on resize.
  useEffect(() => {
    const root = revealRef.current?.parentElement;
    if (!root || reduced) return;
    const measure = () => {
      const width = root.clientWidth;
      if (window.innerWidth < TRAIL_MIN_WIDTH) {
        setGeometry(null);
        return;
      }
      const rootTop = root.getBoundingClientRect().top;
      const points = Array.from(root.querySelectorAll<HTMLElement>('[data-trail]'), (section) => {
        const r = section.getBoundingClientRect();
        const side = (section.dataset.trail as TrailSide | undefined) ?? 'center';
        return { x: (TRAIL_SIDE_X[side] ?? TRAIL_SIDE_X.center) * width, y: r.top - rootTop + TRAIL_ANCHOR_Y * r.height };
      });
      if (points.length < 2) return;
      const d = catmullRomPath(points);
      setGeometry((prev) => (prev?.d === d && prev.width === width ? prev : { d, width, height: root.scrollHeight }));
    };
    let timer = 0;
    const observer = new ResizeObserver(() => {
      window.clearTimeout(timer);
      timer = window.setTimeout(measure, TRAIL_RESIZE_DEBOUNCE_MS);
    });
    observer.observe(root);
    measure();
    return () => {
      observer.disconnect();
      window.clearTimeout(timer);
    };
  }, [reduced]);

  // Reveal on scroll.
  useEffect(() => {
    const reveal = revealRef.current;
    const inner = innerRef.current;
    const path = pathRef.current;
    const dot = dotRef.current;
    const root = reveal?.parentElement;
    if (!geometry || !reveal || !inner || !path || !root) return;
    const length = path.getTotalLength();
    const xs = new Float32Array(TRAIL_SAMPLES + 1);
    const ys = new Float32Array(TRAIL_SAMPLES + 1);
    for (let i = 0; i <= TRAIL_SAMPLES; i++) {
      const pt = path.getPointAtLength((length * i) / TRAIL_SAMPLES);
      xs[i] = pt.x;
      ys[i] = pt.y;
    }
    const height = geometry.height;

    let frame = 0;
    const draw = () => {
      const y = Math.min(height, Math.max(0, window.innerHeight / 2 - root.getBoundingClientRect().top));
      // A window whose bottom edge sits at y: the outer layer moves up, the inner one back
      // down by the same amount, so the line stays put and only the part above y shows.
      reveal.style.transform = `translate3d(0, ${y - height}px, 0)`;
      inner.style.transform = `translate3d(0, ${height - y}px, 0)`;
      if (dot) {
        const lo = lowerIndex(ys, y);
        const hi = Math.min(TRAIL_SAMPLES, lo + 1);
        const span = ys[hi]! - ys[lo]!;
        const f = Math.min(1, Math.max(0, span > 0 ? (y - ys[lo]!) / span : 0));
        const u = (lo + f) / TRAIL_SAMPLES;
        dot.style.transform = `translate3d(${xs[lo]! + (xs[hi]! - xs[lo]!) * f}px, ${ys[lo]! + (ys[hi]! - ys[lo]!) * f}px, 0)`;
        dot.style.opacity = u > 0.01 && u < 0.99 ? '1' : '0';
      }
    };
    const onScroll = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(draw);
    };
    draw();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', onScroll);
      cancelAnimationFrame(frame);
    };
  }, [geometry]);

  return { revealRef, innerRef, pathRef, dotRef, geometry };
}
