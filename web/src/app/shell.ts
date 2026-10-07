import { Component, inject } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { AuthStore } from './core/auth.store';

@Component({
  selector: 'bk-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive],
  template: `
    <header class="border-b border-slate-200 bg-white">
      <nav class="mx-auto flex max-w-6xl flex-wrap items-center gap-4 px-4 py-3 text-sm">
        <a routerLink="/" class="text-lg font-bold text-slate-900">Briefklar</a>
        @if (!auth.isDemo()) { <a routerLink="/neu" routerLinkActive="font-semibold text-brand-700">Neuer Brief</a> }
        <a routerLink="/beispiele" routerLinkActive="font-semibold text-brand-700">Beispiele</a>
        @if (auth.isAdmin()) { <a routerLink="/admin" routerLinkActive="font-semibold text-brand-700">Benutzer</a> }
        <span class="ml-auto text-slate-500">{{ auth.user()?.name }}</span>
        @if (!auth.isDemo()) { <a routerLink="/konto" routerLinkActive="font-semibold text-brand-700">Konto</a> }
        <button type="button" class="text-slate-600 hover:text-brand-700" (click)="logout()">Abmelden</button>
      </nav>
    </header>
    <main class="mx-auto max-w-6xl px-4 py-8"><router-outlet /></main>
    <footer class="mx-auto max-w-6xl px-4 pb-8 text-xs text-slate-500">Keine Rechtsberatung. Im Zweifel bei der Behörde nachfragen.</footer>
  `,
})
export class Shell {
  protected readonly auth = inject(AuthStore);
  private readonly router = inject(Router);

  protected logout(): void {
    this.auth.clear();
    void this.router.navigateByUrl('/login');
  }
}
