'use client';

import { useId, useRef, useState } from 'react';

import { PARTY_LIMITS } from '@/shared/const';
import { useDismiss } from '@/shared/hooks';
import { formatParty } from '@/shared/lib/text';
import { Stepper } from '@/shared/ui';

import styles from './StaySearchForm.module.css';

interface Party {
  adults: number;
  children: number;
  rooms: number;
}

interface GuestsPickerProps {
  party: Party;
  onChange: (party: Party) => void;
}

export function GuestsPicker({ party, onChange }: GuestsPickerProps) {
  const { adults, children, rooms } = party;
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const panelId = useId();
  useDismiss(ref, open, () => setOpen(false));

  const summary = `${formatParty(adults, children)}${rooms > 1 ? ` · ${rooms} rooms` : ''}`;
  return (
    <div ref={ref} className={styles.cell}>
      <span className={styles.cellLabel}>GUESTS</span>
      <button type="button" className={styles.cellButton} aria-expanded={open} aria-controls={panelId} onClick={() => setOpen((o) => !o)}>
        {summary}
      </button>
      {open && (
        <div id={panelId} className={styles.popover}>
          <div className={styles.panelRow}>
            <span>
              Adults <small>12 and over</small>
            </span>
            <Stepper
              label="Adults"
              value={adults}
              min={PARTY_LIMITS.adults.min}
              max={PARTY_LIMITS.adults.max}
              onChange={(v) => onChange({ adults: v, children, rooms })}
            />
          </div>
          <div className={styles.panelRow}>
            <span>
              Children <small>up to 11</small>
            </span>
            <Stepper
              label="Children"
              value={children}
              min={PARTY_LIMITS.children.min}
              max={PARTY_LIMITS.children.max}
              onChange={(v) => onChange({ adults, children: v, rooms })}
            />
          </div>
          <div className={styles.panelRow}>
            <span>Rooms</span>
            <Stepper
              label="Rooms"
              value={rooms}
              min={PARTY_LIMITS.rooms.min}
              max={PARTY_LIMITS.rooms.max}
              onChange={(v) => onChange({ adults, children, rooms: v })}
            />
          </div>
          <button type="button" className={styles.panelDone} onClick={() => setOpen(false)}>
            Done
          </button>
        </div>
      )}
    </div>
  );
}
