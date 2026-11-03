import Image from 'next/image';

import type { Photo } from '@/apis/catalog';

import styles from './Gallery.module.css';

/** Large first photo with the rest beside it; a swipeable strip on phones. */
export function Gallery({ photos, name }: { photos: readonly Photo[]; name: string }) {
  if (photos.length === 0) return null;
  return (
    <div className={styles.gallery} data-count={Math.min(photos.length, 3)} aria-label={`Photos of the ${name}`} role="group">
      {photos.map((photo, i) => (
        <div key={photo.url} className={styles.item}>
          <Image
            src={photo.url}
            alt={photo.alt}
            fill
            priority={i === 0}
            sizes={i === 0 ? '(max-width: 700px) 100vw, 66vw' : '(max-width: 700px) 100vw, 33vw'}
            className={styles.photo}
          />
        </div>
      ))}
    </div>
  );
}
