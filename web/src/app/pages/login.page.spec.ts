import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideHttpClient } from '@angular/common/http';
import { LoginPage } from './login.page';

describe('LoginPage', () => {
  it('renders the disclaimer and the demo credentials', () => {
    TestBed.configureTestingModule({ providers: [provideRouter([]), provideHttpClient()] });
    const fixture = TestBed.createComponent(LoginPage);
    fixture.detectChanges();
    const text: string = fixture.nativeElement.textContent;
    expect(text).toContain('Keine Rechtsberatung. Im Zweifel bei der Behörde nachfragen.');
    expect(text).toContain('demo@briefklar.app');
    expect(text).toContain('demo-briefklar');
  });
});
