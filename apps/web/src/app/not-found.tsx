import { PageShell } from '@/features/site';
import { ROUTES } from '@/shared/const';
import { ButtonLink, EmptyState } from '@/shared/ui';

export default function NotFound() {
  return (
    <PageShell>
      <EmptyState title="We couldn't find that page" action={<ButtonLink href={ROUTES.home}>Back to the estate</ButtonLink>}>
        The link may be old, or the page may have moved.
      </EmptyState>
    </PageShell>
  );
}
