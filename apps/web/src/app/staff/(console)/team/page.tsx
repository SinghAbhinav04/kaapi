import type { Metadata } from 'next';

import { TeamPage } from '@/features/team';

export const metadata: Metadata = { title: 'Team & access' };

export default function Page() {
  return <TeamPage />;
}
