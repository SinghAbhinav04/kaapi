import type { Metadata } from 'next';

import { RatesPage } from '@/features/rates-admin';

export const metadata: Metadata = { title: 'Rates & allotment' };

export default function Page() {
  return <RatesPage />;
}
