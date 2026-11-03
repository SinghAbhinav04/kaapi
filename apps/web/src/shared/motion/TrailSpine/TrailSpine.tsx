'use client';

import { useId } from 'react';

import { useTrailSpine } from './hooks';
import styles from './TrailSpine.module.css';

/**
 * A line that draws itself through the page as you scroll, with a glowing dot at its tip.
 * Place it as the first child of a `position: relative` container whose sections carry
 * data-trail="left" | "center" | "right".
 */
export function TrailSpine() {
  const { svgRef, pathRef, dotRef, geometry } = useTrailSpine();
  const glow = `trail-glow-${useId().replace(/:/g, '')}`;
  const width = geometry?.width ?? 1;
  const height = geometry?.height ?? 1;
  return (
    <svg
      ref={svgRef}
      aria-hidden="true"
      className={styles.svg}
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      style={{ display: geometry ? 'block' : 'none' }}
    >
      <defs>
        <filter id={glow} x="-200%" y="-200%" width="500%" height="500%">
          <feGaussianBlur stdDeviation="5" />
        </filter>
      </defs>
      {geometry && (
        <>
          <path ref={pathRef} className={styles.path} d={geometry.d} />
          <circle ref={dotRef} className={styles.dot} r="5" filter={`url(#${glow})`} />
        </>
      )}
    </svg>
  );
}
