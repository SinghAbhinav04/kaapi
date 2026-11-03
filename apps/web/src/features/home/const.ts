import { ROUTES } from '@/shared/const';

export const HERO_IMAGES = {
  ridge: '/images/site/hero-bg.avif',
  ridgeMobile: '/images/site/hero-bg-mobile.avif',
  rows: '/images/site/hero-coffee-rows.avif',
  flora: '/images/site/hero-fg.avif',
} as const;

/** The scroll story's lines, in order. The last one stays. */
export const HERO_BEATS = [
  'Five hours from Bengaluru, the signal drops.',
  'By four, the mist fills the valley.',
  'Your filter coffee was grown on the slope below your room.',
] as const;

export const WHY_DIRECT = [
  { strong: 'Our best rate, GST included.', rest: 'The price you see on this page is the price you pay.' },
  { strong: 'Free cancellation up to 7 days before arrival', rest: 'on flexible rates.' },
  { strong: 'A morning estate walk', rest: 'and a roastery tasting come with every breakfast rate.' },
] as const;

export const ESTATE_DAY = [
  { time: '07:00', title: 'Guided estate walk', body: 'Through the coffee and pepper, past the pulping unit, ending with a tasting at the roastery.' },
  {
    time: '13:00',
    title: 'Malnad kitchen',
    body: 'Akki rotti, kadubu and pandi curry, with South Indian, North Indian and continental options. Jain and vegan meals on request.',
  },
  { time: '16:00', title: 'Mist hour', body: 'The valley fills with cloud most afternoons. Find a window seat and a filter coffee.' },
] as const;

export const ESTATE_IMAGES = {
  cherries: { src: '/images/site/coffee-cherries.avif', alt: 'Ripe red and green coffee cherries on the branch' },
  harvest: { src: '/images/site/harvest.avif', alt: 'Hands picking ripe coffee cherries' },
  coffee: { src: '/images/site/filter-coffee.avif', alt: 'Filter coffee being poured from a brass tumbler into a davara on a verandah ledge' },
} as const;

export const EVENTS_IMAGE = '/images/site/estate-lawn-wedding.avif';

export const VENUES = [
  { capacity: 400, name: 'ESTATE LAWN' },
  { capacity: 120, name: 'VALLEY DECK' },
  { capacity: 80, name: "PLANTERS' HALL" },
] as const;

export const GETTING_HERE = [
  { distance: '~40 KM', text: 'Kadur railway station. We can arrange a pick-up.' },
  { distance: '~150 KM', text: 'Mangaluru airport.' },
  { distance: '5–6 H', text: 'By road from Bengaluru. Signal drops in the last 10 km, so download the map before you leave.' },
] as const;

/**
 * Where the map points. The brief names only Chikkamagaluru; the estate's exact pin is a
 * labelled assumption until the resort confirms it.
 */
const MAP_QUERY = 'Baba Budangiri, Chikkamagaluru, Karnataka';
export const MAP = {
  label: 'Baba Budan Hills',
  sub: 'Chikkamagaluru, Karnataka',
  /** Google's keyless embed, loaded only when the guest asks for it. */
  embedSrc: `https://maps.google.com/maps?q=${encodeURIComponent(MAP_QUERY)}&z=11&output=embed`,
  /** Opens the Google Maps app on phones, the website elsewhere. */
  openHref: `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(MAP_QUERY)}`,
} as const;

export const ENQUIRE_EVENT_HREF = `${ROUTES.enquire}?kind=event`;

/** "1 left" shows when this few or fewer rooms remain on the website. */
export const SCARCITY_THRESHOLD = 2;

/** id of the stays section, the target after a search. */
export const STAYS_ANCHOR = 'stays';

/** The plan whose total a stay card shows (flexible, room only). */
export const HEADLINE_PLAN = 'RO-FLEX';
