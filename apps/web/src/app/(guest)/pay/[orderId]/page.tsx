import type { Metadata } from 'next';

import { PaymentPage } from '@/features/payment';

export const metadata: Metadata = { title: 'Payment', robots: { index: false } };

export default async function Page({ params }: { params: Promise<{ orderId: string }> }) {
  const { orderId } = await params;
  return <PaymentPage orderId={orderId} />;
}
