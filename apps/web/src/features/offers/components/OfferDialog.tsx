'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';

import { useRooms } from '@/apis/catalog';
import { useCampaigns, useCreateOffer, useUpdateOffer, type Offer } from '@/apis/offers';
import { Button, Checkbox, SelectField, TextField } from '@/shared/ui';

import { COPY, TYPE_OPTIONS } from '../const';
import { toForm, toRequest } from '../lib';
import type { OfferForm } from '../types';
import styles from './Offers.module.css';

interface OfferDialogProps {
  /** null: closed. An empty object: a new offer. */
  editing: { offer?: Offer } | null;
  onClose: () => void;
}

/** Create or change an offer. Pausing is the Active switch; nothing is deleted, since bookings point at it. */
export function OfferDialog({ editing, onClose }: OfferDialogProps) {
  const dialog = useRef<HTMLDialogElement>(null);
  const open = editing !== null;
  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  return (
    <dialog ref={dialog} className={styles.dialog} onClose={onClose} aria-labelledby="offer-title">
      {/* Keyed so each opening starts from that offer's values. */}
      {editing && <OfferFormBody key={editing.offer?.id ?? 'new'} offer={editing.offer} onDone={onClose} />}
    </dialog>
  );
}

function OfferFormBody({ offer, onDone }: { offer?: Offer; onDone: () => void }) {
  const [form, setForm] = useState<OfferForm>(() => toForm(offer));
  const rooms = useRooms();
  const subscribers = useCampaigns().data?.subscribers ?? 0;
  const create = useCreateOffer();
  const update = useUpdateOffer();
  const mutation = offer ? update : create;
  const set = <K extends keyof OfferForm>(key: K, value: OfferForm[K]) => setForm((f) => ({ ...f, [key]: value }));
  const field = (k: string) => mutation.error?.fieldErrors[k];

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const body = toRequest(form);
    if (offer) update.mutate({ id: offer.id, body }, { onSuccess: onDone });
    else create.mutate(body, { onSuccess: onDone });
  };
  const toggleRoom = (id: string, on: boolean) => set('roomTypeIds', on ? [...form.roomTypeIds, id] : form.roomTypeIds.filter((r) => r !== id));

  return (
    <form className={styles.dialogBody} onSubmit={submit}>
      <h2 id="offer-title" className={styles.dialogTitle}>
        {offer ? `Edit ${offer.code}` : COPY.add}
      </h2>
      <div className={styles.grid2}>
        <TextField
          label="Code"
          value={form.code}
          onChange={(e) => set('code', e.target.value.toUpperCase())}
          error={field('code')}
          hint="Shown on the quote, e.g. DIWALI26"
          maxLength={24}
          required
        />
        <div className={styles.amount}>
          <SelectField label="Type" value={form.type} onChange={(e) => set('type', e.target.value as OfferForm['type'])} options={TYPE_OPTIONS} />
          <TextField
            label={form.type === 'PERCENT' ? 'Percent' : 'Rupees'}
            type="number"
            inputMode="decimal"
            min="0"
            step="any"
            value={form.amount}
            onChange={(e) => set('amount', e.target.value)}
            error={field('value')}
            required
          />
        </div>
      </div>
      <TextField
        label="Description"
        value={form.description}
        onChange={(e) => set('description', e.target.value)}
        error={field('description')}
        hint="What guests read, e.g. Diwali on the estate: 15% off"
        maxLength={200}
        required
      />
      <fieldset className={styles.fieldset}>
        <legend>Stays it covers</legend>
        <div className={styles.grid2}>
          <TextField
            label="First night"
            type="date"
            value={form.stayFrom}
            onChange={(e) => set('stayFrom', e.target.value)}
            error={field('stay_from')}
            required
          />
          <TextField label="Last night" type="date" value={form.stayTo} onChange={(e) => set('stayTo', e.target.value)} error={field('stay_to')} required />
        </div>
      </fieldset>
      <fieldset className={styles.fieldset}>
        <legend>Bookable</legend>
        <div className={styles.grid2}>
          <TextField label="From" type="date" value={form.bookFrom} onChange={(e) => set('bookFrom', e.target.value)} error={field('book_from')} required />
          <TextField label="Until" type="date" value={form.bookTo} onChange={(e) => set('bookTo', e.target.value)} error={field('book_to')} required />
        </div>
      </fieldset>
      <div className={styles.grid2}>
        <TextField
          label="Minimum nights"
          type="number"
          min="1"
          max="30"
          value={form.minNights}
          onChange={(e) => set('minNights', e.target.value)}
          error={field('min_nights')}
        />
        <TextField
          label="Use limit"
          optional
          type="number"
          min="1"
          value={form.maxUses}
          onChange={(e) => set('maxUses', e.target.value)}
          error={field('max_uses')}
          hint={offer ? `Used ${offer.used_count} times. ${COPY.usesHint}` : COPY.usesHint}
        />
      </div>
      <fieldset className={styles.fieldset}>
        <legend>Room types</legend>
        <p className={styles.muted}>{COPY.roomsHint}</p>
        <div className={styles.rooms}>
          {(rooms.data?.rooms ?? []).map((r) => (
            <Checkbox key={r.id} checked={form.roomTypeIds.includes(r.id)} onChange={(e) => toggleRoom(r.id, e.target.checked)}>
              {r.name}
            </Checkbox>
          ))}
        </div>
        {field('room_type_ids') && <p className={styles.error}>{field('room_type_ids')}</p>}
      </fieldset>
      <Checkbox checked={form.autoApply} onChange={(e) => set('autoApply', e.target.checked)}>
        Apply automatically <span className={styles.muted}>· {COPY.autoHint}</span>
      </Checkbox>
      <Checkbox checked={form.active} onChange={(e) => set('active', e.target.checked)}>
        Active <span className={styles.muted}>· untick to pause</span>
      </Checkbox>
      <Checkbox checked={form.announce && form.active} disabled={!form.active} onChange={(e) => set('announce', e.target.checked)}>
        Email it <span className={styles.muted}>· {COPY.announce(subscribers)}</span>
      </Checkbox>
      {mutation.isError && Object.keys(mutation.error.fieldErrors).length === 0 && (
        <p className={styles.error} role="alert">
          {mutation.error.message}
        </p>
      )}
      <div className={styles.row}>
        <Button variant="quiet" onClick={onDone}>
          Cancel
        </Button>
        <Button type="submit" loading={mutation.isPending}>
          {offer ? 'Save' : 'Add offer'}
        </Button>
      </div>
    </form>
  );
}
