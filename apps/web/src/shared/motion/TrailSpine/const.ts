/** Points sampled along the path, for mapping scroll position to path length. */
export const TRAIL_SAMPLES = 300;

/** Below this width the line would cross content, so it isn't drawn. */
export const TRAIL_MIN_WIDTH = 900;

/** Where each [data-trail] section pulls the line, as a share of the width. */
export const TRAIL_SIDE_X = { left: 0.12, center: 0.5, right: 0.88 } as const;

/** The line passes each section a third of the way down it. */
export const TRAIL_ANCHOR_Y = 0.35;

export const TRAIL_RESIZE_DEBOUNCE_MS = 160;
