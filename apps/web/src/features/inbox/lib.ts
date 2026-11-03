import type { EnquirySummary } from '@/apis/enquiries';

import { QUOTE_HEAD, QUOTE_LINE } from './const';
import type { DraftBlock, ItemBadge, QuoteRow } from './types';

/**
 * Splits a draft into paragraphs and the quote block the server inserted, so the quote can
 * be shown as "live from the rate table". Text that doesn't match stays plain text.
 */
export function parseDraft(body: string): DraftBlock[] {
  const blocks: DraftBlock[] = [];
  let quote: { head: string | null; rows: QuoteRow[] } | null = null;
  let text: string[] = [];
  const flushText = () => {
    const t = text.join('\n').trim();
    if (t) for (const para of t.split(/\n{2,}/)) blocks.push({ kind: 'text', text: para.trim() });
    text = [];
  };
  const flushQuote = () => {
    if (quote && quote.rows.length) blocks.push({ kind: 'quote', ...quote });
    else if (quote?.head) text.push(quote.head);
    quote = null;
  };
  for (const line of body.split('\n')) {
    const row = QUOTE_LINE.exec(line.trim());
    if (QUOTE_HEAD.test(line.trim())) {
      flushText();
      quote = { head: line.trim(), rows: [] };
    } else if (row) {
      if (!quote) {
        flushText();
        quote = { head: null, rows: [] };
      }
      quote.rows.push({ room: row[1] ?? '', roomOnly: row[2] ?? '', breakfast: row[3] ?? null, left: row[4] ? Number(row[4]) : null });
    } else {
      flushQuote();
      text.push(line);
    }
  }
  flushQuote();
  flushText();
  return blocks;
}

/** The one badge an inbox row shows, most urgent first. */
export function itemBadge(e: EnquirySummary): ItemBadge | null {
  if (e.category === 'COMPLAINT' && e.status === 'NEW') return { label: 'Complaint · GM alerted', tone: 'alert' };
  if (e.ai_status === 'PENDING') return { label: 'AI reading…', tone: 'neutral' };
  if (e.status === 'REPLIED') return { label: 'Replied', tone: 'neutral' };
  if (e.needs_check) return { label: 'AI unsure · check details', tone: 'amber' };
  if (e.has_draft && e.status !== 'CLOSED') return { label: 'Draft ready', tone: 'estate' };
  return null;
}

export const confidencePercent = (c: number | null | undefined) => (c === null || c === undefined ? null : Math.round(c * 100));

/** Open complaints, and how long the oldest has waited. */
export function complaintSummary(list: readonly EnquirySummary[]) {
  const open = list.filter((e) => e.category === 'COMPLAINT' && (e.status === 'NEW' || e.status === 'IN_PROGRESS'));
  const oldest = open.reduce<string | null>((min, e) => (min === null || e.received_at < min ? e.received_at : min), null);
  return { count: open.length, oldest };
}
