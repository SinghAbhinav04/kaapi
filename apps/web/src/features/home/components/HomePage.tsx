import { SiteFooter, SiteHeader } from '@/features/site';
import { TrailSpine } from '@/shared/motion';

import styles from './HomePage.module.css';
import { Hero } from './Hero';
import { EstateSection, EventsSection, GettingHere, WhyDirect } from './Sections';
import { StaysSection } from './StaysSection';

export function HomePage() {
  return (
    <div className={styles.page}>
      <SiteHeader variant="glass" />
      <main>
        <Hero />
        <div className={styles.trail}>
          <TrailSpine />
          <WhyDirect />
          <StaysSection />
          <EstateSection />
          <EventsSection />
          <GettingHere />
          <SiteFooter />
        </div>
      </main>
    </div>
  );
}
