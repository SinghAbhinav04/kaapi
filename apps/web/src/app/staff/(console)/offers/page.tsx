import type { Metadata } from 'next';

import { OffersPage } from '@/features/offers';

export const metadata: Metadata = { title: 'Offers' };

export default function Page() {
  return <OffersPage />;
}
