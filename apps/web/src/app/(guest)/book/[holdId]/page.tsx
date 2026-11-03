import type { Metadata } from 'next';

import { CheckoutPage } from '@/features/checkout';

export const metadata: Metadata = { title: 'Your details', robots: { index: false } };

/** Step 2. Client-only: the hold token lives in this tab's sessionStorage. */
export default async function Page({ params }: { params: Promise<{ holdId: string }> }) {
  const { holdId } = await params;
  return <CheckoutPage holdId={holdId} />;
}
