import { Urgency } from '../core/models';

/** Strict YYYY-MM-DD that is a real calendar date ("2026-02-31" is invalid). */
export function isValidDate(d: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(d)) return false;
  const [y, m, day] = d.split('-').map(Number);
  const t = new Date(Date.UTC(y, m - 1, day));
  return t.getUTCFullYear() === y && t.getUTCMonth() === m - 1 && t.getUTCDate() === day;
}

/** Days from today to the deadline; 0 for an invalid deadline (never NaN). */
export function daysUntil(deadline: string, today: Date): number {
  if (!isValidDate(deadline)) return 0;
  const [y, m, d] = deadline.split('-').map(Number);
  const target = Date.UTC(y, m - 1, d);
  const now = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate());
  return Math.round((target - now) / 86_400_000);
}

/** The model's value, except: less than 7 days left (or overdue) is always red. An invalid deadline counts as none. */
export function urgencyFor(deadline: string | null, given: Urgency, today: Date): Urgency {
  if (deadline && isValidDate(deadline) && daysUntil(deadline, today) < 7) return 'red';
  return given;
}
