import type { Booking } from '@/apis/booking';
import { SITE } from '@/shared/const';
import { addDays } from '@/shared/lib/dates';

const ics = (date: string) => date.replaceAll('-', '');
const escape = (text: string) => text.replace(/[\\;,]/g, (m) => `\\${m}`).replace(/\n/g, '\\n');

/** An all-day calendar event for the stay (RFC 5545). Built in the browser; nothing is sent. */
export function stayCalendarFile(booking: Booking): string {
  const lines = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//Kaapi Estate//Booking//EN',
    'BEGIN:VEVENT',
    `UID:${booking.ref}@kaapiestate`,
    `DTSTAMP:${new Date().toISOString().replace(/[-:]/g, '').slice(0, 15)}Z`,
    `DTSTART;VALUE=DATE:${ics(booking.stay.check_in)}`,
    `DTEND;VALUE=DATE:${ics(addDays(booking.stay.check_out, 1))}`,
    `SUMMARY:${escape(`${SITE.name}: ${booking.room.name}`)}`,
    `LOCATION:${escape(`${SITE.name}, Chikkamagaluru, Karnataka`)}`,
    `DESCRIPTION:${escape(`Booking ${booking.ref}. Check-in from ${SITE.checkIn}, check-out by ${SITE.checkOut}.`)}`,
    'END:VEVENT',
    'END:VCALENDAR',
  ];
  return lines.join('\r\n');
}

export function downloadFile(name: string, content: string, type: string): void {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const a = Object.assign(document.createElement('a'), { href: url, download: name });
  a.click();
  URL.revokeObjectURL(url);
}
