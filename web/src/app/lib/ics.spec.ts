import { buildIcs } from './ics';

describe('buildIcs', () => {
  const ics = buildIcs({
    title: 'Frist: Ausländerbehörde', date: '2026-11-15',
    description: 'Unterlagen einreichen; Pass, Foto\nund Mietvertrag',
    now: new Date('2026-10-07T10:00:00Z'), uid: 'test-1',
  });

  it('is an all-day event with CRLF line endings', () => {
    expect(ics.startsWith('BEGIN:VCALENDAR\r\n')).toBe(true);
    expect(ics).toContain('DTSTART;VALUE=DATE:20261115\r\n');
    expect(ics).toContain('DTEND;VALUE=DATE:20261116\r\n');
    expect(ics).toContain('DTSTAMP:20261007T100000Z\r\n');
    expect(ics).toContain('UID:test-1@briefklar.omarfourati.de\r\n');
    expect(ics.trimEnd().endsWith('END:VCALENDAR')).toBe(true);
  });

  it('escapes special characters and adds a reminder 3 days before', () => {
    expect(ics).toContain('DESCRIPTION:Unterlagen einreichen\\; Pass\\, Foto\\nund Mietvertrag');
    expect(ics).toContain('TRIGGER:-P3D');
  });

  it('escapes a lone carriage return', () => {
    const r = buildIcs({ title: 't', date: '2026-11-15', description: 'a\rb\r\nc', uid: 'u' });
    expect(r).toContain('DESCRIPTION:a\\nb\\nc');
  });

  it('folds long lines at 75 octets', () => {
    const long = buildIcs({ title: 'x'.repeat(200), date: '2026-11-15', description: '', uid: 'u' });
    for (const line of long.split('\r\n')) {
      expect(new TextEncoder().encode(line).length).toBeLessThanOrEqual(75);
    }
  });

  it('folds multi-byte text without breaking characters', () => {
    for (const title of ['Ä'.repeat(100), 'ع'.repeat(60)]) {
      const r = buildIcs({ title, date: '2026-11-15', description: '', uid: 'u' });
      for (const line of r.split('\r\n')) {
        expect(new TextEncoder().encode(line).length).toBeLessThanOrEqual(75);
      }
      expect(r).not.toContain('�');
      const summary = r.split('\r\n ').join('').split('\r\n').find((l) => l.startsWith('SUMMARY:'));
      expect(summary).toBe('SUMMARY:' + title);
    }
  });

  it('handles month ends', () => {
    expect(buildIcs({ title: 't', date: '2026-12-31', description: '', uid: 'u' })).toContain('DTEND;VALUE=DATE:20270101');
  });

  it('rejects invalid dates', () => {
    for (const date of ['2026-02-31', '2026-13-01', 'morgen', '']) {
      expect(() => buildIcs({ title: 't', date, description: '', uid: 'u' })).toThrowError('Ungültiges Datum');
    }
  });
});
