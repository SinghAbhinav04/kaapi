import type { Channel } from '@/apis/enquiries';

/** Small glyph for where a message came from: WhatsApp bubble, envelope, or globe. */
export function ChannelIcon({ channel }: { channel: Channel }) {
  if (channel === 'WHATSAPP') {
    return (
      <svg width="10" height="10" viewBox="0 0 24 24" fill="var(--estate)" aria-hidden="true">
        <path d="M12 2a10 10 0 00-8.7 15l-1.3 5 5.1-1.3A10 10 0 1012 2z" />
      </svg>
    );
  }
  if (channel === 'EMAIL') {
    return (
      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="var(--ink)" strokeWidth="2.5" aria-hidden="true">
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <path d="M3 7l9 6 9-6" />
      </svg>
    );
  }
  return (
    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="var(--ink)" strokeWidth="2.5" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3a14 14 0 010 18M12 3a14 14 0 000 18" />
    </svg>
  );
}
