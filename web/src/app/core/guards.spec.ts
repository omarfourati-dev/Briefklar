import { TestBed } from '@angular/core/testing';
import { ActivatedRouteSnapshot, CanActivateFn, Router, RouterStateSnapshot, UrlTree, provideRouter } from '@angular/router';
import { AuthStore } from './auth.store';
import { adminGuard, authGuard, notDemoGuard } from './guards';
import { LoginResponse, Role } from './models';

const login = (role: Role, expiresAt = '2099-01-01T00:00:00Z'): LoginResponse => ({
  token: 't', expiresAt, user: { email: 'a@b.de', name: 'A', role },
});

describe('guards', () => {
  let auth: AuthStore;
  let router: Router;

  beforeEach(() => {
    sessionStorage.clear();
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    auth = TestBed.inject(AuthStore);
    router = TestBed.inject(Router);
  });

  const run = (g: CanActivateFn) =>
    TestBed.runInInjectionContext(() => g({} as ActivatedRouteSnapshot, {} as RouterStateSnapshot));
  const path = (r: unknown) => router.serializeUrl(r as UrlTree);

  it('authGuard: no session -> /login', () => {
    expect(path(run(authGuard))).toBe('/login');
  });
  it('authGuard: session -> true', () => {
    auth.set(login('user'));
    expect(run(authGuard)).toBe(true);
  });
  it('authGuard: expired in-memory session -> /login', () => {
    auth.set(login('user', '2000-01-01T00:00:00Z'));
    expect(path(run(authGuard))).toBe('/login');
  });

  it('adminGuard: admin -> true; user, demo and no session -> /', () => {
    auth.set(login('admin'));
    expect(run(adminGuard)).toBe(true);
    auth.set(login('user'));
    expect(path(run(adminGuard))).toBe('/');
    auth.set(login('demo'));
    expect(path(run(adminGuard))).toBe('/');
    auth.clear();
    expect(path(run(adminGuard))).toBe('/');
  });

  it('notDemoGuard: demo -> /beispiele; user, admin and no session -> true', () => {
    auth.set(login('demo'));
    expect(path(run(notDemoGuard))).toBe('/beispiele');
    auth.set(login('user'));
    expect(run(notDemoGuard)).toBe(true);
    auth.set(login('admin'));
    expect(run(notDemoGuard)).toBe(true);
    auth.clear();
    expect(run(notDemoGuard)).toBe(true);
  });
});
