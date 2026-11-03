'use client';

import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';

import { useMe } from '@/apis/auth';
import { ROUTES } from '@/shared/const';

/**
 * Signed-out visitors go to the login page (and come back after). The API enforces every
 * permission itself; this only spares staff a page of 401s.
 */
export function useStaffGuard() {
  const me = useMe();
  const router = useRouter();
  const pathname = usePathname();
  const signedOut = me.isSuccess && me.data === null;

  useEffect(() => {
    if (signedOut) router.replace(`${ROUTES.staff.login}?next=${encodeURIComponent(pathname)}`);
  }, [signedOut, router, pathname]);

  const user = me.data ?? null;
  return { me, user, isStaff: !!user && user.role !== 'GUEST' };
}
