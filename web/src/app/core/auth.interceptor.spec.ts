import { HttpClient, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { AuthStore } from './auth.store';
import { authInterceptor } from './auth.interceptor';
import { LoginResponse } from './models';

const session = (token: string): LoginResponse => ({
  token, expiresAt: '2099-01-01T00:00:00Z', user: { email: 'a@b.de', name: 'A', role: 'user' },
});

describe('authInterceptor', () => {
  let http: HttpClient;
  let ctl: HttpTestingController;
  let auth: AuthStore;
  let navigate: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    sessionStorage.clear();
    TestBed.configureTestingModule({
      providers: [provideRouter([]), provideHttpClient(withInterceptors([authInterceptor])), provideHttpClientTesting()],
    });
    http = TestBed.inject(HttpClient);
    ctl = TestBed.inject(HttpTestingController);
    auth = TestBed.inject(AuthStore);
    navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
  });

  afterEach(() => ctl.verify());

  it('sets the Bearer header when a token exists', () => {
    auth.set(session('tok'));
    http.get('/api/users').subscribe();
    const r = ctl.expectOne('/api/users');
    expect(r.request.headers.get('Authorization')).toBe('Bearer tok');
    r.flush([]);
  });

  it('sends no header without a token', () => {
    http.get('/api/users').subscribe();
    const r = ctl.expectOne('/api/users');
    expect(r.request.headers.has('Authorization')).toBe(false);
    r.flush([]);
  });

  it('clears the session and redirects on 401 with a token', () => {
    auth.set(session('tok'));
    http.get('/api/users').subscribe({ error: () => undefined });
    ctl.expectOne('/api/users').flush({}, { status: 401, statusText: 'Unauthorized' });
    expect(auth.token()).toBeNull();
    expect(navigate).toHaveBeenCalledWith(['/login'], { queryParams: { abgelaufen: 1 } });
  });

  it('leaves everything untouched on 401 without a token', () => {
    http.get('/api/users').subscribe({ error: () => undefined });
    ctl.expectOne('/api/users').flush({}, { status: 401, statusText: 'Unauthorized' });
    expect(navigate).not.toHaveBeenCalled();
  });

  it('ignores a late 401 from an old token after a new login', () => {
    auth.set(session('old'));
    http.get('/api/users').subscribe({ error: () => undefined });
    const r = ctl.expectOne('/api/users');
    auth.set(session('new'));
    r.flush({}, { status: 401, statusText: 'Unauthorized' });
    expect(auth.token()).toBe('new');
    expect(navigate).not.toHaveBeenCalled();
  });

  it('does not attach the header to or handle 401 of the login call', () => {
    auth.set(session('tok'));
    let status = 0;
    http.post('/api/auth/login', {}).subscribe({ error: (e) => (status = e.status) });
    const r = ctl.expectOne('/api/auth/login');
    expect(r.request.headers.has('Authorization')).toBe(false);
    r.flush({}, { status: 401, statusText: 'Unauthorized' });
    expect(status).toBe(401);
    expect(auth.token()).toBe('tok');
    expect(navigate).not.toHaveBeenCalled();
  });
});
