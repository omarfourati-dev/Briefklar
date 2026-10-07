import { isValidDate } from './urgency';

/** Builds an iCalendar file in the browser – the deadline never goes back to the server. */
export function buildIcs(o: { title: string; date: string; description: string; now?: Date; uid?: string }): string {
  if (!isValidDate(o.date)) throw new Error('Ungültiges Datum');
  const [y, m, d] = o.date.split('-').map(Number);
  const start = new Date(Date.UTC(y, m - 1, d));
  const end = new Date(start.getTime() + 24 * 60 * 60 * 1000);
  const day = (x: Date) => x.toISOString().slice(0, 10).replaceAll('-', '');
  const stamp = (o.now ?? new Date()).toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '');
  const uid = o.uid ?? crypto.randomUUID();
  const lines = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//Briefklar//DE', 'CALSCALE:GREGORIAN', 'METHOD:PUBLISH',
    'BEGIN:VEVENT',
    `UID:${uid}@briefklar.omarfourati.de`,
    `DTSTAMP:${stamp}`,
    `DTSTART;VALUE=DATE:${day(start)}`,
    `DTEND;VALUE=DATE:${day(end)}`,
    `SUMMARY:${escape(o.title)}`,
    `DESCRIPTION:${escape(o.description)}`,
    'BEGIN:VALARM', 'ACTION:DISPLAY', `DESCRIPTION:${escape(o.title)}`, 'TRIGGER:-P3D', 'END:VALARM',
    'END:VEVENT', 'END:VCALENDAR',
  ];
  return lines.map(fold).join('\r\n') + '\r\n';
}

function escape(s: string): string {
  return s.replace(/\\/g, '\\\\').replace(/;/g, '\\;').replace(/,/g, '\\,').replace(/\r\n|\r|\n/g, '\\n');
}

// RFC 5545 §3.1: lines longer than 75 octets continue on the next line, starting with a space.
function fold(line: string): string {
  const enc = new TextEncoder();
  const out: string[] = [];
  let current = '';
  for (const ch of line) {
    const limit = out.length === 0 ? 75 : 74;
    if (enc.encode(current + ch).length > limit) {
      out.push(current);
      current = ch;
    } else {
      current += ch;
    }
  }
  out.push(current);
  return out.join('\r\n ');
}
