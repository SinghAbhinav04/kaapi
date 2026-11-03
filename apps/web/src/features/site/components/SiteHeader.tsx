import Link from 'next/link';

import { ROUTES } from '@/shared/const';
import { cx } from '@/shared/lib/cx';
import { ButtonLink, Logo } from '@/shared/ui';

import { NAV_LINKS } from '../const';
import type { SiteHeaderProps } from '../types';
import styles from './SiteHeader.module.css';

export function SiteHeader({ variant = 'solid' }: SiteHeaderProps) {
  return (
    <header className={cx(styles.header, styles[variant])}>
      <nav aria-label="Main" className={styles.nav}>
        <Logo />
        <div className={styles.links}>
          {NAV_LINKS.map((link) => (
            <Link key={link.href} href={link.href}>
              {link.label}
            </Link>
          ))}
        </div>
        <div className={styles.actions}>
          <Link href={ROUTES.myBookings} className={styles.myBooking}>
            MY BOOKING
          </Link>
          <ButtonLink href={ROUTES.stays} variant="ink" pill>
            Book direct
          </ButtonLink>
        </div>
      </nav>
    </header>
  );
}
