import type { ReactNode } from 'react';

import { SiteFooter } from './SiteFooter';
import { SiteHeader } from './SiteHeader';
import styles from './PageShell.module.css';

/** Header, a centred column and the footer: the frame for simple guest pages. */
export function PageShell({ children, width = 'narrow' }: { children: ReactNode; width?: 'narrow' | 'wide' }) {
  return (
    <>
      <SiteHeader />
      <main className={width === 'wide' ? styles.wide : styles.narrow}>{children}</main>
      <SiteFooter />
    </>
  );
}
