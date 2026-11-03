'use client';

import { StaySearchForm, useStaySearch, type StaySearch } from '@/features/stay-search';

import { STAYS_ANCHOR } from '../const';

/** The hero's search bar. Submitting updates the URL and brings the guest to the prices. */
export function HeroSearch() {
  const { search, setSearch } = useStaySearch();
  const onSubmit = (next: StaySearch) => {
    setSearch(next);
    document.getElementById(STAYS_ANCHOR)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };
  // Re-mount the form when the URL's search changes, so its draft starts from the new values.
  return <StaySearchForm key={`${search.checkIn}|${search.checkOut}|${search.adults}|${search.children}|${search.rooms}`} initial={search} onSubmit={onSubmit} />;
}
