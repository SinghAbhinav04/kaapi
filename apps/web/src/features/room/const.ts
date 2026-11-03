/** Pre-selected plan, as on the design: breakfast is how most guests book. */
export const DEFAULT_PLAN_CODE = 'BB-FLEX';

/** The calendar shows room-only prices before GST, like the staff grid. */
export const CALENDAR_PLAN = 'RO-FLEX';

/** Show "· 1 left" on calendar days and the summary at or below this. */
export const SCARCITY_THRESHOLD = 2;

/** How far ahead the calendar can page (matches the bookable window). */
export const CALENDAR_MAX_MONTHS_AHEAD = 6;

export const PROMO_PATTERN = /^[A-Z0-9]{3,20}$/;
