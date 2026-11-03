import type { HTMLAttributes } from 'react';

import type { PILL_TONES } from './const';

export type PillTone = (typeof PILL_TONES)[number];

export interface PillProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: PillTone;
  /** Space Mono, letter-spaced: for data (confidence, scarcity, codes). */
  mono?: boolean;
}
