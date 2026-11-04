import type { Metadata } from 'next';

import { PageShell } from '@/features/site';
import { UnsubscribePage } from '@/features/unsubscribe';

export const metadata: Metadata = { title: 'Unsubscribe from offers', robots: { index: false } };

export default async function Page({ searchParams }: { searchParams: Promise<{ token?: string }> }) {
  const { token } = await searchParams;
  return (
    <PageShell>
      <UnsubscribePage token={token ?? ''} />
    </PageShell>
  );
}
