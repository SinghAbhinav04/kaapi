'use client';

import { useTrailSpine } from './hooks';
import styles from './TrailSpine.module.css';

/**
 * A line that draws itself through the page as you scroll, with a glowing dot at its tip.
 * Place it as the first child of a `position: relative` container whose sections carry
 * data-trail="left" | "center" | "right".
 *
 * The line is painted once. Scroll only moves two counter-translated layers (a sliding
 * window over the line) and the dot, all transforms, so the compositor does the work and
 * nothing page-tall is repainted per frame.
 */
export function TrailSpine() {
  const { revealRef, innerRef, pathRef, dotRef, geometry } = useTrailSpine();
  const width = geometry?.width ?? 1;
  const height = geometry?.height ?? 1;
  const box = { width, height, display: geometry ? 'block' : 'none' };
  return (
    <>
      <div ref={revealRef} aria-hidden="true" className={styles.reveal} style={box}>
        <div ref={innerRef} className={styles.inner}>
          <svg className={styles.svg} width={width} height={height} viewBox={`0 0 ${width} ${height}`}>
            {geometry && <path ref={pathRef} className={styles.path} d={geometry.d} />}
          </svg>
        </div>
      </div>
      {geometry && <div ref={dotRef} aria-hidden="true" className={styles.dot} />}
    </>
  );
}
