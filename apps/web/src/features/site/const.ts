import { ROUTES } from '@/shared/const';

export const NAV_LINKS = [
  { href: ROUTES.stays, label: 'Stays' },
  { href: '/#estate', label: 'The estate' },
  { href: ROUTES.events, label: 'Weddings & offsites' },
  { href: '/#getting-here', label: 'Getting here' },
] as const;

export const FOOTER_LINKS = [
  { href: ROUTES.privacy, label: 'Privacy' },
  { href: ROUTES.enquire, label: 'Contact' },
  { href: ROUTES.staff.login, label: 'Staff sign-in' },
] as const;

/** Honest labelling of the placeholder imagery (see apps/web/assets/source/CREDITS.md). */
export const IMAGE_CREDIT =
  "Imagery is placeholder until the resort's own shoot. Estate, villa, coffee and event scenes were AI-generated with Google Gemini. Room photos are from Unsplash (Kawê Rodrigues, Sanju Pandita, ann svanidze, Clay Banks).";
