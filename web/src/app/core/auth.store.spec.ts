import { TestBed } from '@angular/core/testing';
import { AuthStore } from './auth.store';

describe('AuthStore', () => {
  beforeEach(() => sessionStorage.clear());

  it('stores the session and derives roles', () => {
    const store = TestBed.inject(AuthStore);
    expect(store.user()).toBeNull();
    store.set({ token: 't', expiresAt: '2099-01-01T00:00:00Z', user: { email: 'demo@briefklar.app', name: 'Demo', role: 'demo' } });
    expect(store.token()).toBe('t');
    expect(store.isDemo()).toBe(true);
    expect(store.isAdmin()).toBe(false);
    expect(sessionStorage.getItem('briefklar.session')).toContain('"token":"t"');
    store.clear();
    expect(store.user()).toBeNull();
    expect(sessionStorage.getItem('briefklar.session')).toBeNull();
  });

  it('restores a valid stored session on construction', () => {
    sessionStorage.setItem('briefklar.session', JSON.stringify({ token: 'kept', expiresAt: '2099-01-01T00:00:00Z', user: { email: 'a', name: 'a', role: 'admin' } }));
    const store = TestBed.inject(AuthStore);
    expect(store.token()).toBe('kept');
    expect(store.isAdmin()).toBe(true);
  });

  it('treats a session expired in memory as logged out', () => {
    const store = TestBed.inject(AuthStore);
    store.set({ token: 't', expiresAt: '2000-01-01T00:00:00Z', user: { email: 'a', name: 'a', role: 'user' } });
    expect(store.token()).toBeNull();
  });

  it('ignores an expired stored session', () => {
    sessionStorage.setItem('briefklar.session', JSON.stringify({ token: 'old', expiresAt: '2000-01-01T00:00:00Z', user: { email: 'a', name: 'a', role: 'user' } }));
    const store = TestBed.inject(AuthStore);
    expect(store.token()).toBeNull();
  });
});
