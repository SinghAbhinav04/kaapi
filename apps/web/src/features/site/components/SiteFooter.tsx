import Link from 'next/link';

import { SITE } from '@/shared/const';

import { FOOTER_LINKS, IMAGE_CREDIT } from '../const';
import styles from './SiteFooter.module.css';

export function SiteFooter() {
  return (
    <footer data-trail="center" className={styles.footer}>
      <div className={styles.row}>
        <span className={styles.brand}>KAAPI ESTATE</span>
        <span>
          Check-in {SITE.checkIn} · Check-out {SITE.checkOut} · We never ask for passport details online
        </span>
        <nav aria-label="Footer" className={styles.links}>
          {FOOTER_LINKS.map((link) => (
            <Link key={link.href} href={link.href}>
              {link.label}
            </Link>
          ))}
        </nav>
      </div>
      <p className={styles.credit}>{IMAGE_CREDIT}</p>
    </footer>
  );
}
