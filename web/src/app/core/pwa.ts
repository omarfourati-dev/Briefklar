import { Injectable, signal } from '@angular/core';

/**
 * Installable app: registers public/sw.js and reports when a new version is waiting. Every deploy ships a new
 * service worker (the build stamps it), the banner in App lets the user switch over with one click.
 */
@Injectable({
  providedIn: 'root',
  useFactory: () => new Pwa(globalThis.navigator?.serviceWorker, () => location.reload()),
})
export class Pwa {
  readonly updateReady = signal(false);
  readonly offline = signal(globalThis.navigator?.onLine === false);
  private registration: ServiceWorkerRegistration | null = null;
  private requested = false;
  private reloading = false;

  constructor(
    private readonly container: ServiceWorkerContainer | undefined,
    private readonly reload: () => void,
  ) {
    globalThis.addEventListener?.('online', () => this.offline.set(false));
    globalThis.addEventListener?.('offline', () => this.offline.set(true));
  }

  async register(): Promise<void> {
    if (!this.container) return;
    const reg = await this.container.register('sw.js', { scope: './' });
    this.registration = reg;
    // no controller yet = first install: nothing to update, the page already runs the newest code
    const hasController = () => this.container?.controller != null;
    if (reg.waiting && hasController()) this.updateReady.set(true);
    reg.addEventListener('updatefound', () => {
      const worker = reg.installing;
      worker?.addEventListener('statechange', () => {
        if (worker.state === 'installed' && hasController()) this.updateReady.set(true);
      });
    });
    this.container.addEventListener('controllerchange', () => {
      // the first install also changes the controller (clients.claim) – only reload when the user asked
      if (!this.requested || this.reloading) return;
      this.reloading = true;
      this.reload();
    });
  }

  applyUpdate(): void {
    this.requested = true;
    this.registration?.waiting?.postMessage('skip-waiting');
  }
}
