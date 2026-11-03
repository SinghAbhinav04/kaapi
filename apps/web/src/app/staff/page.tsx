import { redirect } from 'next/navigation';

import { ROUTES } from '@/shared/const';

export default function Page() {
  redirect(ROUTES.staff.inbox);
}
