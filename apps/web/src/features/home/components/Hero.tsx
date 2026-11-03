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

        {/* Soft gradient bands, not an SVG blur filter: a filter re-rasterises every frame
            while the bands drift, which is what made the hero stutter. */}
        <div className={styles.mist} aria-hidden="true">
          <div className={cx(styles.band, styles.band1, styles.drift)} />
          <div className={cx(styles.band, styles.band2, styles.driftReverse)} />
          <div className={cx(styles.band, styles.band3, styles.drift)} />
        </div>

        <div className={styles.rows}>
          <div className={styles.cam2}>
            {/* Eager: it fades in mid-scroll, and a first decode at that moment drops frames. */}
            <Image src={HERO_IMAGES.rows} alt="" fill sizes="100vw" quality={60} loading="eager" className={styles.photo} style={{ objectPosition: '50% 55%' }} />
          </div>
        </div>

        <div className={styles.flora} aria-hidden="true">
          <Image src={HERO_IMAGES.flora} alt="" fill sizes="110vw" quality={75} loading="eager" className={styles.floraImg} />
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
