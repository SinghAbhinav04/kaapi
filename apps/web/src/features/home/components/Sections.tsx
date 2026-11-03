import Image from 'next/image';

import { ButtonLink } from '@/shared/ui';

import { ENQUIRE_EVENT_HREF, ESTATE_DAY, ESTATE_IMAGES, EVENTS_IMAGE, GETTING_HERE, VENUES, WHY_DIRECT } from '../const';
import { MapCard } from './MapCard';
import styles from './Sections.module.css';

export function WhyDirect() {
  return (
    <section data-trail="center" aria-label="Why book direct" className={styles.why}>
      <div className={`${styles.whyGrid} reveal`}>
        {WHY_DIRECT.map((item, i) => (
          <div key={item.strong} className={styles.whyItem}>
            <span className={styles.index}>{String(i + 1).padStart(2, '0')}</span>
            <p>
              <strong>{item.strong}</strong> {item.rest}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}

export function EstateSection() {
  return (
    <section data-trail="right" id="estate" aria-labelledby="estate-title" className={styles.estate}>
      <div className={styles.estateGrid}>
        <div className={styles.estateIntro}>
          <div className="reveal">
            <p className="label">THE ESTATE</p>
            <h2 id="estate-title" className={styles.heading}>
              Coffee from the plant <span className="accent">to your cup.</span>
            </h2>
          </div>
          <div className={styles.pair}>
            <div className={`${styles.portrait} reveal`}>
              <Image src={ESTATE_IMAGES.cherries.src} alt={ESTATE_IMAGES.cherries.alt} fill sizes="(max-width: 700px) 50vw, 25vw" className="parallax" />
            </div>
            <div className={`${styles.portrait} ${styles.offset} reveal`}>
              <Image src={ESTATE_IMAGES.harvest.src} alt={ESTATE_IMAGES.harvest.alt} fill sizes="(max-width: 700px) 50vw, 25vw" className="parallax" />
            </div>
          </div>
        </div>
        <ol className={styles.day}>
          {ESTATE_DAY.map((item) => (
            <li key={item.time} className={`${styles.dayItem} reveal`}>
              <span className={styles.time}>{item.time}</span>
              <div>
                <h3 className={styles.dayTitle}>{item.title}</h3>
                <p className={styles.dayBody}>{item.body}</p>
              </div>
            </li>
          ))}
        </ol>
        <figure className={`${styles.wide} reveal`}>
          <Image src={ESTATE_IMAGES.coffee.src} alt={ESTATE_IMAGES.coffee.alt} fill sizes="100vw" className="parallax" />
          <figcaption className={styles.caption}>16:00 · FILTER COFFEE ON THE VERANDAH</figcaption>
        </figure>
      </div>
    </section>
  );
}

export function EventsSection() {
  return (
    <section data-trail="left" id="events" aria-labelledby="events-title" className={styles.events}>
      <Image src={EVENTS_IMAGE} alt="" fill sizes="100vw" className={`${styles.eventsPhoto} parallax`} />
      <div className={styles.eventsShade} aria-hidden="true" />
      <div className={styles.eventsGrid}>
        <div className="reveal">
          <p className={styles.eventsLabel}>WEDDINGS &amp; OFFSITES</p>
          <h2 id="events-title" className={styles.heading}>
            Up to 400 guests on the <span className="accent">Estate Lawn.</span>
          </h2>
          <p className={styles.eventsLead}>
            Tell us your dates, guest count and budget. The right person on our team replies with availability and a quote, usually within 30 minutes during
            working hours.
          </p>
        </div>
        <div className={`${styles.venues} reveal`}>
          {VENUES.map((v) => (
            <div key={v.name} className={styles.venue}>
              <p className={styles.venueNumber}>{v.capacity}</p>
              <p className={styles.venueName}>{v.name}</p>
            </div>
          ))}
          <div className={`${styles.venue} ${styles.venueCta}`}>
            <ButtonLink href={ENQUIRE_EVENT_HREF} variant="light" pill>
              Plan an event
            </ButtonLink>
          </div>
        </div>
      </div>
    </section>
  );
}

export function GettingHere() {
  return (
    <section data-trail="right" id="getting-here" aria-labelledby="getting-title" className={styles.getting}>
      <div className={styles.gettingGrid}>
        <h2 id="getting-title" className={`${styles.heading} reveal`}>
          Getting <span className="accent">here</span>
        </h2>
        <div className={styles.gettingBody}>
          <MapCard />
          <ol className={styles.route}>
            {GETTING_HERE.map((leg) => (
              <li key={leg.distance} className={`${styles.leg} reveal`}>
                <span className={styles.distance}>{leg.distance}</span>
                <span>{leg.text}</span>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </section>
  );
}
