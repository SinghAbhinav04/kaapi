import { cx } from '@/shared/lib/cx';

import styles from './Skeleton.module.css';
import type { SkeletonProps } from './types';

/** A placeholder block with the shape of what's loading, so the layout doesn't jump. */
export function Skeleton({ width = '100%', height = 16, radius, className }: SkeletonProps) {
  return <span aria-hidden="true" className={cx(styles.skeleton, className)} style={{ width, height, borderRadius: radius }} />;
}
