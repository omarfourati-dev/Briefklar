import { Component, inject, isDevMode } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { Pwa } from './core/pwa';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet],
  template: `
    @if (pwa.offline()) {
      <p role="status" class="bg-amber-100 px-4 py-2 text-center text-sm text-amber-900">
        Keine Verbindung – Briefklar braucht Internet für Texterkennung und Erklärung.
      </p>
    }
    @if (pwa.updateReady()) {
      <p role="status" class="flex items-center justify-center gap-3 bg-brand-700 px-4 py-2 text-sm text-white">
        Neue Version von Briefklar verfügbar.
        <button type="button" class="rounded bg-white px-3 py-1 font-semibold text-brand-700" (click)="pwa.applyUpdate()">Neu laden</button>
      </p>
    }
    <router-outlet />
  `,
})
export class App {
  protected readonly pwa = inject(Pwa);

  constructor() {
    if (!isDevMode()) void this.pwa.register().catch(() => undefined); // the app works without it
  }
}
