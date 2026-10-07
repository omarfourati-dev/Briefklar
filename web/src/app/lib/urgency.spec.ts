import { daysUntil, urgencyFor } from './urgency';

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
});
