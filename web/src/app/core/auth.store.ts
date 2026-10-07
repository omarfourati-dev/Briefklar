import { Injectable, computed, signal } from '@angular/core';
import { LoginResponse } from './models';

const KEY = 'briefklar.session';

// sessionStorage: the token is gone when the tab is closed.
@Injectable({ providedIn: 'root' })
export class AuthStore {
  private readonly session = signal<LoginResponse | null>(load());
  readonly user = computed(() => this.session()?.user ?? null);
  readonly token = computed(() => this.session()?.token ?? null);
  readonly isDemo = computed(() => this.user()?.role === 'demo');
  readonly isAdmin = computed(() => this.user()?.role === 'admin');

  set(session: LoginResponse): void {
    this.session.set(session);
    try { sessionStorage.setItem(KEY, JSON.stringify(session)); } catch { /* private mode: memory only */ }
  }

  clear(): void {
    this.session.set(null);
    try { sessionStorage.removeItem(KEY); } catch { /* ignore */ }
  }
}

function load(): LoginResponse | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return null;
    const s = JSON.parse(raw) as LoginResponse;
    return new Date(s.expiresAt).getTime() > Date.now() ? s : null;
  } catch {
    return null;
  }
}
