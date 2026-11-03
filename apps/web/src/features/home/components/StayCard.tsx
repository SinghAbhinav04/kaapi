import Image from 'next/image';
import Link from 'next/link';

import { ApproxPrice } from '@/features/currency';
import { ROUTES } from '@/shared/const';
import { formatINR } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';
import { Pill, Skeleton } from '@/shared/ui';

import { SCARCITY_THRESHOLD } from '../const';
import type { StayCardModel } from '../types';
import styles from './StaysSection.module.css';

interface StayCardProps {
  card: StayCardModel;
  nights: number;
  query: string;
  pricesLoading: boolean;
}

export function StayCard({ card, nights, query, pricesLoading }: StayCardProps) {
  const scarce = card.left !== null && card.left <= SCARCITY_THRESHOLD;
  return (
    <Link href={ROUTES.room(card.slug, query)} className={`${styles.card} reveal`}>
      <div className={styles.media}>
        {card.photo && (
          <Image src={card.photo.url} alt={card.photo.alt} fill sizes="(max-width: 700px) 100vw, (max-width: 1200px) 50vw, 33vw" className={styles.zoom} />
        )}
        <span className={styles.peek} aria-hidden="true">
          View stay
        </span>
        {scarce && (
          <Pill tone="paper" mono className={styles.badge}>
            {card.left} LEFT ON OUR SITE
          </Pill>
        )}
      </div>
      <div className={styles.body}>
        <h3 className={styles.title}>{card.name}</h3>
        <p className={styles.facts}>{card.facts}</p>
        <div className={styles.price}>
          {card.fromPaise !== null ? (
            <>
              <span className={styles.caption}>{plural(nights, 'night')} · incl. GST</span>
              <span className={styles.amount}>{formatINR(card.fromPaise)}</span>
              <ApproxPrice paise={card.fromPaise} />
            </>
          ) : card.note ? (
            <span className={styles.unavailable}>{card.note}</span>
          ) : pricesLoading ? (
            <>
              <Skeleton width={110} height={14} />
              <Skeleton width={90} height={22} />
            </>
          ) : (
            <span className={styles.caption}>Choose dates to see prices</span>
          )}
        </div>
      </div>
    </Link>
  );
}
