import { HttpErrorResponse } from '@angular/common/http';

export function problemMessage(err: unknown, fallback: string): string {
  if (err instanceof HttpErrorResponse && err.error && typeof err.error === 'object' && typeof err.error.detail === 'string') {
    return err.error.detail;
  }
  return fallback;
}
