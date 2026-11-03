import { getImageProps } from 'next/image';
import Image from 'next/image';

import { SITE } from '@/shared/const';
import { cx } from '@/shared/lib/cx';

import { HERO_BEATS, HERO_IMAGES } from '../const';
import styles from './Hero.module.css';
import { HeroSearch } from './HeroSearch';

/**
 * The scroll story: a tall section with a pinned scene. As you scroll, the ridge zooms, the
 * mist thickens, the scene crossfades to the coffee rows and four lines come and go. All of
 * it is CSS scroll-driven animation: no JavaScript runs while scrolling.
 */
export function Hero() {
  // Art direction: a portrait crop on phones, the wide ridge on larger screens.
  const common = { alt: '', fill: true, priority: true, sizes: '100vw', quality: 60 } as const;
  const { props: wide } = getImageProps({ ...common, src: HERO_IMAGES.ridge });
  const { props: tall } = getImageProps({ ...common, src: HERO_IMAGES.ridgeMobile });

  return (
    <section className={styles.wrap} aria-labelledby="hero-title">
      <div className={styles.sticky}>
        <div className={styles.cam}>
          <div className={styles.kenBurns}>
            <picture>
              <source media="(max-width: 700px)" srcSet={tall.srcSet} sizes="100vw" />
              <img {...wide} className={styles.photo} alt="" />
            </picture>
          </div>
        </div>

        <svg className={styles.mist} viewBox="0 0 1440 900" preserveAspectRatio="none" aria-hidden="true">
          <defs>
            <filter id="hero-mist" x="-10%" y="-80%" width="120%" height="260%">
              <feGaussianBlur stdDeviation="22" />
            </filter>
          </defs>
          <g className={styles.drift}>
            <ellipse cx="720" cy="470" rx="980" ry="60" fill="#f4f1ea" filter="url(#hero-mist)" />
          </g>
          <g className={styles.driftReverse}>
            <ellipse cx="560" cy="600" rx="900" ry="70" fill="#f4f1ea" opacity="0.85" filter="url(#hero-mist)" />
          </g>
          <g className={styles.drift}>
            <ellipse cx="900" cy="720" rx="860" ry="60" fill="#f4f1ea" opacity="0.7" filter="url(#hero-mist)" />
          </g>
        </svg>

        <div className={styles.rows}>
          <div className={styles.cam2}>
            <Image src={HERO_IMAGES.rows} alt="" fill sizes="100vw" quality={60} className={styles.photo} style={{ objectPosition: '50% 55%' }} />
          </div>
        </div>

        <div className={styles.flora} aria-hidden="true">
          <Image src={HERO_IMAGES.flora} alt="" fill sizes="110vw" quality={75} className={styles.floraImg} />
        </div>
        <div className={styles.vignette} aria-hidden="true" />
        <div className={styles.glow} aria-hidden="true" />

        <div className={styles.intro}>
          <p className={styles.eyebrow}>{SITE.location.toUpperCase()}</p>
          <h1 id="hero-title" className={styles.title}>
            Wake up inside a <span className={styles.titleAccent}>working coffee estate.</span>
          </h1>
          <p className={styles.lead}>
            64 rooms and 12 villas among the coffee and silver oaks. Book here for our best rate, with GST shown before you pay.
          </p>
        </div>

        <p className={styles.hint} aria-hidden="true">
          SCROLL TO WALK IN
          <svg width="14" height="22" viewBox="0 0 14 22" fill="none" stroke="currentColor" strokeWidth="1.5">
            <rect x="1" y="1" width="12" height="20" rx="6" />
            <path d="M7 6v4" />
          </svg>
        </p>

        {HERO_BEATS.map((line, i) => (
          <div key={line} className={cx(styles.beat, styles[`b${i + 1}`])} aria-hidden="true">
            <p className={styles.beatText}>{line}</p>
          </div>
        ))}
        <div className={cx(styles.beat, styles.b4)} aria-hidden="true">
          <div className={styles.beatFinal}>
            <p className={styles.beatTitle}>
              Book it here, <span className="accent">direct.</span>
            </p>
            <p className={styles.beatSub}>The price you see is the price you pay, GST included.</p>
          </div>
        </div>

        <div className={styles.search}>
          <HeroSearch />
        </div>
      </div>
    </section>
  );
}
