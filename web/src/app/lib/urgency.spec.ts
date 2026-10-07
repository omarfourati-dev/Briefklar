import { daysUntil, isValidDate, urgencyFor } from './urgency';

describe('urgency', () => {
  const today = new Date('2026-10-07T12:00:00');
  it('counts days', () => {
    expect(daysUntil('2026-10-14', today)).toBe(7);
    expect(daysUntil('2026-10-07', today)).toBe(0);
  });
  it('forces red below 7 days or when overdue', () => {
    expect(urgencyFor('2026-10-13', 'green', today)).toBe('red');
    expect(urgencyFor('2026-10-01', 'yellow', today)).toBe('red');
  });
  it('keeps the model value otherwise', () => {
    expect(urgencyFor('2026-12-01', 'yellow', today)).toBe('yellow');
    expect(urgencyFor(null, 'green', today)).toBe('green');
  });
  it('keeps the model value at exactly 7 days', () => {
    expect(urgencyFor('2026-10-14', 'green', today)).toBe('green');
  });
  it('validates dates strictly', () => {
    expect(isValidDate('2026-02-28')).toBe(true);
    expect(isValidDate('2028-02-29')).toBe(true);
    expect(isValidDate('2026-02-31')).toBe(false);
    expect(isValidDate('2026-13-01')).toBe(false);
    expect(isValidDate('2026-1-01')).toBe(false);
    expect(isValidDate('2026-10-07T00:00')).toBe(false);
    expect(isValidDate('')).toBe(false);
  });
  it('treats an invalid deadline like null', () => {
    expect(urgencyFor('2026-02-31', 'yellow', today)).toBe('yellow');
    expect(urgencyFor('garbage', 'green', today)).toBe('green');
    expect(Number.isNaN(daysUntil('garbage', today))).toBe(false);
  });
});
