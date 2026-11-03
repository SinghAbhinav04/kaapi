import type { Metadata } from 'next';

import { ConfirmationPage } from '@/features/confirmation';

export const metadata: Metadata = { title: 'Your booking', robots: { index: false } };

export default async function Page({ params }: { params: Promise<{ ref: string }> }) {
  const { ref } = await params;
  return <ConfirmationPage bookingRef={decodeURIComponent(ref).toUpperCase()} />;
}
