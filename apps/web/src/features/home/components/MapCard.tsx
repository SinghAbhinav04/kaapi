'use client';

import { useState } from 'react';

import { MAP } from '../const';
import styles from './Sections.module.css';

/**
 * A drawn preview of where the estate is. Google's map loads only when the guest taps it,
 * so the home page stays light and nothing is sent to Google before they choose to.
 */
export function MapCard() {
  const [live, setLive] = useState(false);

  return (
    <figure className={styles.map}>
      <div className={styles.mapFrame}>
        {live ? (
          <iframe
            className={styles.mapIframe}
            src={MAP.embedSrc}
            title={`Map: ${MAP.label}, ${MAP.sub}`}
            loading="lazy"
            referrerPolicy="no-referrer-when-downgrade"
            allowFullScreen
          />
        ) : (
          <button type="button" className={styles.mapPreview} onClick={() => setLive(true)} aria-label={`Show the map of ${MAP.label}`}>
            <svg className={styles.mapArt} viewBox="0 0 400 250" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
              <g fill="none" stroke="currentColor" strokeWidth="1">
                <path d="M-10 190 C 60 150, 120 210, 200 170 S 330 120, 410 150" />
                <path d="M-10 160 C 70 120, 130 175, 205 140 S 320 95, 410 118" />
                <path d="M-10 130 C 80 92, 140 140, 210 110 S 315 70, 410 88" />
                <path d="M40 100 C 110 60, 170 98, 225 80 S 300 40, 380 55" />
                <path d="M110 72 C 160 45, 205 70, 240 58 S 290 30, 330 36" />
              </g>
              <path d="M-10 238 C 90 228, 150 205, 205 128" fill="none" stroke="currentColor" strokeWidth="2.5" strokeDasharray="6 5" />
            </svg>
            <span className={styles.mapPin} aria-hidden="true">
              <svg width="28" height="36" viewBox="0 0 28 36">
                <path d="M14 35s12-12.2 12-21A12 12 0 0 0 2 14c0 8.8 12 21 12 21z" fill="var(--estate)" />
                <circle cx="14" cy="14" r="4.5" fill="var(--paper)" />
              </svg>
            </span>
            <span className={styles.mapCta}>Tap to open the map</span>
          </button>
        )}
      </div>
      <figcaption className={styles.mapCaption}>
        <span>
          <strong>{MAP.label}</strong> · {MAP.sub}
        </span>
        <a href={MAP.openHref} target="_blank" rel="noopener noreferrer" className={styles.mapLink}>
          Open in Google Maps
        </a>
      </figcaption>
    </figure>
  );
}
