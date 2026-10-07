import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthStore } from './auth.store';

export const authGuard: CanActivateFn = () =>
  inject(AuthStore).token() ? true : inject(Router).createUrlTree(['/login']);

export const adminGuard: CanActivateFn = () =>
  inject(AuthStore).isAdmin() ? true : inject(Router).createUrlTree(['/']);

// The demo account sees only the examples (no uploads, no AI costs).
export const notDemoGuard: CanActivateFn = () =>
  inject(AuthStore).isDemo() ? inject(Router).createUrlTree(['/beispiele']) : true;
