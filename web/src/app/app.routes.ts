import { inject } from '@angular/core';
import { Routes } from '@angular/router';
import { AuthStore } from './core/auth.store';
import { adminGuard, authGuard, notDemoGuard } from './core/guards';
import { Shell } from './shell';

export const routes: Routes = [
  { path: 'login', loadComponent: () => import('./pages/login.page').then((m) => m.LoginPage) },
  {
    path: '', component: Shell, canActivate: [authGuard],
    children: [
      { path: '', pathMatch: 'full', redirectTo: () => (inject(AuthStore).isDemo() ? 'beispiele' : 'neu') },
      { path: 'neu', canActivate: [notDemoGuard], loadComponent: () => import('./pages/letter.page').then((m) => m.LetterPage) },
      { path: 'beispiele', loadComponent: () => import('./pages/examples.page').then((m) => m.ExamplesPage) },
      { path: 'konto', canActivate: [notDemoGuard], loadComponent: () => import('./pages/account.page').then((m) => m.AccountPage) },
      { path: 'admin', canActivate: [adminGuard], loadComponent: () => import('./pages/admin.page').then((m) => m.AdminPage) },
    ],
  },
  { path: '**', redirectTo: '' },
];
