'use client';

import { useEffect, useRef, useState } from 'react';

import { useReducedMotion } from '@/shared/hooks/useMediaQuery';

import { TRAIL_ANCHOR_Y, TRAIL_MIN_WIDTH, TRAIL_RESIZE_DEBOUNCE_MS, TRAIL_SAMPLES, TRAIL_SIDE_X } from './const';
import { catmullRomPath, lowerIndex } from './geometry';
import type { TrailGeometry, TrailSide } from './types';

/**
 * Measures the [data-trail] sections inside the svg's parent and returns the path. Scroll
 * then draws the line and moves the dot by writing to the DOM directly: no state updates
 * while scrolling, so no React renders. Only a resize re-measures.
 */
export function useTrailSpine() {
  const svgRef = useRef<SVGSVGElement>(null);
  const pathRef = useRef<SVGPathElement>(null);
  const dotRef = useRef<SVGCircleElement>(null);
  const [geometry, setGeometry] = useState<TrailGeometry | null>(null);
  const reduced = useReducedMotion();

  // Measure on mount and on resize.
  useEffect(() => {
    const root = svgRef.current?.parentElement;
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

  // Draw on scroll.
  useEffect(() => {
    const path = pathRef.current;
    const dot = dotRef.current;
    const root = svgRef.current?.parentElement;
    if (!geometry || !path || !root) return;
    const length = path.getTotalLength();
    const xs = new Float32Array(TRAIL_SAMPLES + 1);
    const ys = new Float32Array(TRAIL_SAMPLES + 1);
    for (let i = 0; i <= TRAIL_SAMPLES; i++) {
      const pt = path.getPointAtLength((length * i) / TRAIL_SAMPLES);
      xs[i] = pt.x;
      ys[i] = pt.y;
    }
    path.style.strokeDasharray = String(length);

    let frame = 0;
    const draw = () => {
      const y = window.innerHeight / 2 - root.getBoundingClientRect().top;
      const lo = lowerIndex(ys, y);
      const hi = Math.min(TRAIL_SAMPLES, lo + 1);
      const span = ys[hi]! - ys[lo]!;
      const f = Math.min(1, Math.max(0, span > 0 ? (y - ys[lo]!) / span : 0));
      const u = Math.min(1, Math.max(0, (lo + f) / TRAIL_SAMPLES));
      path.style.strokeDashoffset = String(length * (1 - u));
      if (dot) {
        dot.setAttribute('cx', String(xs[lo]! + (xs[hi]! - xs[lo]!) * f));
        dot.setAttribute('cy', String(ys[lo]! + (ys[hi]! - ys[lo]!) * f));
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

  return { svgRef, pathRef, dotRef, geometry };
}
