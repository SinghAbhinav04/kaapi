'use client';

import { useState, type FormEvent } from 'react';

import { useCampaigns, useCreateCampaign, type CampaignRequest } from '@/apis/offers';
import { Card } from '@/features/staff-shell';
import { formatDayMonth } from '@/shared/lib/dates';
import { Button, EmptyState, ErrorState, Pill, SelectField, Skeleton, TextAreaField, TextField } from '@/shared/ui';

import { CAMPAIGN_KIND_LABEL, CAMPAIGN_KINDS, CAMPAIGN_STATUS, COPY } from '../const';
import styles from './Offers.module.css';

/** Offer and seasonal emails: what went out, and a form for a festive or announcement note. */
export function CampaignsCard() {
  const campaigns = useCampaigns();
  const create = useCreateCampaign();
  const [kind, setKind] = useState<CampaignRequest['kind']>('FESTIVE');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const field = (k: string) => (create.isError ? create.error.fieldErrors[k] : undefined);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate(
      { kind, subject: subject.trim(), body: body.trim() },
      {
        onSuccess: () => {
          setSubject('');
          setBody('');
        },
      },
    );
  };

  return (
    <Card className={styles.card}>
      <h2 className={styles.cardTitle}>{COPY.campaignsTitle}</h2>
      <p className={styles.muted}>{COPY.campaignsLead(campaigns.data?.subscribers ?? 0)}</p>
      {campaigns.isPending ? (
        <Skeleton height={120} />
      ) : campaigns.isError ? (
        <ErrorState
          message={campaigns.error.message}
          requestId={campaigns.error.requestId}
          onRetry={() => void campaigns.refetch()}
          retrying={campaigns.isFetching}
        />
      ) : campaigns.data.campaigns.length === 0 ? (
        <EmptyState title="No offer emails" compact>
          {COPY.campaignsEmpty}
        </EmptyState>
      ) : (
        <div className={styles.scroller}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th scope="col">Email</th>
                <th scope="col">Kind</th>
                <th scope="col">Sent to</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {campaigns.data.campaigns.map((c) => {
                const status = CAMPAIGN_STATUS[c.status];
                return (
                  <tr key={c.id}>
                    <td>
                      {c.subject}
                      <span className={styles.sub}>{formatDayMonth((c.sent_at ?? c.send_at).slice(0, 10))}</span>
                    </td>
                    <td>{CAMPAIGN_KIND_LABEL[c.kind]}</td>
                    <td className="mono">{c.recipients}</td>
                    <td>
                      <Pill tone={status.tone} mono>
                        {status.label}
                      </Pill>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <form className={styles.campaignForm} onSubmit={submit}>
        <SelectField label="Kind" value={kind} onChange={(e) => setKind(e.target.value as CampaignRequest['kind'])} options={CAMPAIGN_KINDS} />
        <TextField label="Subject" value={subject} onChange={(e) => setSubject(e.target.value)} maxLength={150} error={field('subject')} required />
        <TextAreaField label="Message" rows={4} value={body} onChange={(e) => setBody(e.target.value)} maxLength={4000} error={field('body')} required />
        {create.isSuccess && (
          <p className={styles.muted} role="status">
            {COPY.campaignSent}
          </p>
        )}
        {create.isError && Object.keys(create.error.fieldErrors).length === 0 && (
          <p className={styles.error} role="alert">
            {create.error.message}
          </p>
        )}
        <Button type="submit" loading={create.isPending} disabled={subject.trim().length < 3 || body.trim().length < 10}>
          Send to subscribers
        </Button>
      </form>
    </Card>
  );
}
