import { Urgency } from '../core/models';

export function daysUntil(deadline: string, today: Date): number {
  const [y, m, d] = deadline.split('-').map(Number);
  const target = Date.UTC(y, m - 1, d);
  const now = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate());
  return Math.round((target - now) / 86_400_000);
}

/** The model's value, except: less than 7 days left (or overdue) is always red. */
export function urgencyFor(deadline: string | null, given: Urgency, today: Date): Urgency {
  if (deadline && daysUntil(deadline, today) < 7) return 'red';
  return given;
}
