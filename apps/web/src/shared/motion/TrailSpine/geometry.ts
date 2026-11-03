import type { Point } from './types';

/**
 * A smooth cubic Bézier path through every point (Catmull-Rom converted to Bézier), like
 * isocodelabs.com's TrailSpine. Pure, so it is easy to test.
 */
export function catmullRomPath(points: readonly Point[]): string {
  const first = points[0];
  if (!first) return '';
  let d = `M ${first.x} ${first.y}`;
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[Math.max(0, i - 1)]!;
    const p1 = points[i]!;
    const p2 = points[i + 1]!;
    const p3 = points[Math.min(points.length - 1, i + 2)]!;
    const c1 = { x: p1.x + (p2.x - p0.x) / 6, y: p1.y + (p2.y - p0.y) / 6 };
    const c2 = { x: p2.x - (p3.x - p1.x) / 6, y: p2.y - (p3.y - p1.y) / 6 };
    d += ` C ${c1.x} ${c1.y}, ${c2.x} ${c2.y}, ${p2.x} ${p2.y}`;
  }
  return d;
}

/** Index in a sorted array of the last sample above `y` (binary search). */
export function lowerIndex(ys: Float32Array, y: number): number {
  let lo = 0;
  let hi = ys.length - 1;
  while (hi - lo > 1) {
    const mid = (lo + hi) >> 1;
    if (ys[mid]! < y) lo = mid;
    else hi = mid;
  }
  return lo;
}
