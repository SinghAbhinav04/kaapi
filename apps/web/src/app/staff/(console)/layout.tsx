import type { Metadata } from 'next';
import { Suspense, type ReactNode } from 'react';

import { StaffShell } from '@/features/staff-shell';

export const metadata: Metadata = { title: { default: 'Staff', template: '%s · Staff · Kaapi Estate' }, robots: { index: false } };

/** Every console page is client-rendered behind the guard; the API checks permissions on each call. */
export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return (
    <Suspense>
      <StaffShell>{children}</StaffShell>
    </Suspense>
  );
}
