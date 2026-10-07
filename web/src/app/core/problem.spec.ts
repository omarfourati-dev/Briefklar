import { HttpErrorResponse } from '@angular/common/http';
import { problemMessage } from './problem';

describe('problemMessage', () => {
  it('returns the problem+json detail', () => {
    const err = new HttpErrorResponse({ status: 400, error: { title: 'x', detail: 'Falsches Passwort.' } });
    expect(problemMessage(err, 'Fallback')).toBe('Falsches Passwort.');
  });
  it('falls back without a detail or for other errors', () => {
    expect(problemMessage(new HttpErrorResponse({ status: 500, error: 'boom' }), 'Fallback')).toBe('Fallback');
    expect(problemMessage(new HttpErrorResponse({ status: 500, error: {} }), 'Fallback')).toBe('Fallback');
    expect(problemMessage(new Error('x'), 'Fallback')).toBe('Fallback');
  });
});
