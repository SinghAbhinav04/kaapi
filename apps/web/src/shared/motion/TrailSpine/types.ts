import type { TRAIL_SIDE_X } from './const';

export type TrailSide = keyof typeof TRAIL_SIDE_X;

export interface Point {
  x: number;
  y: number;
}

export interface TrailGeometry {
  d: string;
  width: number;
  height: number;
}
