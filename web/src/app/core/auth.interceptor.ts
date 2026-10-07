import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthStore } from './auth.store';

const LOGIN_URL = '/api/auth/login';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  // Login carries no token and its 401 means "wrong credentials", not "session expired".
  if (req.url === LOGIN_URL) return next(req);
  const auth = inject(AuthStore);
  const router = inject(Router);
  const token = auth.token();
  const request = token ? req.clone({ setHeaders: { Authorization: `Bearer ${token}` } }) : req;
  return next(request).pipe(
    catchError((err: HttpErrorResponse) => {
      // Only the session that sent the request may be ended (a late 401 must not kill a newer login).
      if (err.status === 401 && token && auth.token() === token) {
        auth.clear();
        void router.navigate(['/login'], { queryParams: { abgelaufen: 1 } });
      }
      return throwError(() => err);
    }),
  );
};
