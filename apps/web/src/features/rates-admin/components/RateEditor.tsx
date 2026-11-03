'use client';

import { useState } from 'react';

import type { UpdateRatesRequest, UpdateRatesResponse } from '@/apis/rates';
import type { ApiError } from '@/apis/http';
import type { RateNight, RateRow } from '@/apis/rates';
import { formatDay } from '@/shared/lib/dates';
import { plural } from '@/shared/lib/text';
import { Button, Segmented, Stepper } from '@/shared/ui';

import { MIN_STAY_OPTIONS } from '../const';
import { initialEdit, minimumAllotment, toUpdate } from '../lib';
import type { GridSelection, RateEdit } from '../types';
import styles from './Rates.module.css';

interface RateEditorProps {
  row: RateRow;
  nights: readonly RateNight[];
  selection: GridSelection;
  canPrice: boolean;
  canInventory: boolean;
  pricePlanShown: boolean;
  onCancel: () => void;
  onSave: (body: UpdateRatesRequest) => void;
  saving: boolean;
  error: ApiError | null;
  saved: UpdateRatesResponse | undefined;
}

/** Edits the selected nights. Keyed by the selection, so it resets when the selection changes. */
export function RateEditor({ row, nights, selection, canPrice, canInventory, pricePlanShown, onCancel, onSave, saving, error, saved }: RateEditorProps) {
  const [start] = useState<RateEdit>(() => initialEdit(nights));
  const [edit, setEdit] = useState<RateEdit>(start);
  const floor = minimumAllotment(nights);
  const held = nights.reduce((s, n) => s + n.held, 0);
  const body = toUpdate(selection, start, edit);
  const range = selection.first === selection.last ? formatDay(selection.first) : `${formatDay(selection.first)} – ${formatDay(selection.last)}`;

  return (
    <section aria-labelledby="sel-title" className={styles.editor}>
      <div className={styles.editorTitle}>
        <p>{plural(nights.length, 'night')} selected</p>
        <h2 id="sel-title">
          {row.name} · {range}
        </h2>
      </div>

      <label className={styles.field}>
        Room-only price per night
        <span className={styles.money}>
          <span>₹</span>
          <input
            inputMode="numeric"
            value={edit.priceRupees}
            placeholder="mixed"
            disabled={!canPrice || !pricePlanShown}
            onChange={(e) => setEdit({ ...edit, priceRupees: e.target.value.replace(/[^\d]/g, '') })}
          />
        </span>
      </label>

      <div className={styles.field}>
        Rooms online
        <Stepper
          label="Rooms online"
          tone="dark"
          value={edit.allotment ?? nights[0]?.allotment ?? 0}
          min={floor}
          max={row.total_keys}
          warn={(edit.allotment ?? 0) <= floor && floor > 0}
          onChange={(v) => canInventory && setEdit({ ...edit, allotment: v })}
        />
      </div>

      <div className={styles.field}>
        Minimum stay
        <Segmented
          label="Minimum stay"
          tone="dark"
          options={MIN_STAY_OPTIONS}
          value={String(edit.minStay ?? '')}
          onChange={(v) => canInventory && setEdit({ ...edit, minStay: Number(v) })}
        />
      </div>

      <label className={styles.switch}>
        <input type="checkbox" role="switch" checked={!!edit.stopSell} disabled={!canInventory} onChange={(e) => setEdit({ ...edit, stopSell: e.target.checked })} />
        Stop sell
      </label>

      <div className={styles.editorActions}>
        <button type="button" className={styles.darkButton} onClick={onCancel}>
          Cancel
        </button>
        <Button variant="light" onClick={() => body && onSave(body)} loading={saving} disabled={!body}>
          Save {plural(nights.length, 'night')}
        </Button>
      </div>

      {(error || floor > 0 || !pricePlanShown || !canPrice || saved) && (
        <p role={error ? 'alert' : 'status'} className={error ? styles.editorError : styles.editorNote}>
          {error
            ? error.message
            : saved
              ? `Saved ${plural(saved.nights_changed, 'night')}. Breakfast and non-refundable prices followed.`
              : !canPrice
                ? 'Your role can change rooms and restrictions, not prices.'
                : !pricePlanShown
                  ? 'Switch to Room only to change prices; the other plans follow it.'
                  : `${plural(floor, 'room')} already sold${held ? ` or on hold (${held} at checkout now)` : ''}, so at least ${floor} must stay online.`}
        </p>
      )}
    </section>
  );
}
