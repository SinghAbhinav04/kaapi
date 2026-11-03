import type { Metadata } from 'next';
import { Suspense } from 'react';

import { StaffLoginPage } from '@/features/staff-login';

export const metadata: Metadata = { title: 'Staff sign-in', robots: { index: false } };

export default function Page() {
  return (
    <Suspense>
      <StaffLoginPage />
    </Suspense>
  );
}
