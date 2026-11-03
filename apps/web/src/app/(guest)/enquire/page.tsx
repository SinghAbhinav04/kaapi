import type { Metadata } from 'next';

import { EnquirePage } from '@/features/enquire';

export const metadata: Metadata = { title: 'Ask us or plan an event' };

interface Props {
  searchParams: Promise<{ kind?: string }>;
}

export default async function Page({ searchParams }: Props) {
  const { kind } = await searchParams;
  return <EnquirePage initialKind={kind === 'event' ? 'EVENT' : 'GENERAL'} />;
}
