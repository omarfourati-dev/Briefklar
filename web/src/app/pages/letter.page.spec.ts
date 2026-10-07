import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { LetterPage } from './letter.page';
import { EXAMPLES } from '../lib/examples';

describe('LetterPage', () => {
  beforeEach(() => TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] }));

  it('previews, lets the user redact a word, explains and restores the values', async () => {
    const fixture = TestBed.createComponent(LetterPage);
    const http = TestBed.inject(HttpTestingController);
    const el: HTMLElement = fixture.nativeElement;
    fixture.detectChanges();

    const textarea = el.querySelector('textarea') as HTMLTextAreaElement;
    textarea.value = 'Sehr geehrter Herr Benali, Ihr Nachbar Okafor bittet um Rückruf bis 15.11.2026.';
    textarea.dispatchEvent(new Event('input'));
    fixture.detectChanges(); // enables the button
    (el.querySelector('[data-testid=preview]') as HTMLButtonElement).click();
    http.expectOne('/api/letters/preview').flush({
      text: 'Sehr geehrter Herr [NAME_1], Ihr Nachbar Okafor bittet um Rückruf bis 15.11.2026.',
      findings: [{ placeholder: '[NAME_1]', kind: 'NAME', value: 'Benali' }], inputKind: 'text',
    });
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.textContent).not.toContain('Benali');
    const okafor = [...el.querySelectorAll('[data-word]')].find((s) => s.textContent === 'Okafor') as HTMLElement;
    okafor.click();
    fixture.detectChanges();
    (el.querySelector('[data-testid=explain]') as HTMLButtonElement).click();

    const req = http.expectOne('/api/letters/explain');
    expect(req.request.body.text).toContain('[MANUELL_1]');
    expect(req.request.body.text).not.toContain('Okafor');
    req.flush({
      explanation: { ...EXAMPLES[0].explanation, replyDraft: 'Gruß an [NAME_1] und [MANUELL_1]' },
      findings: [],
    });
    await fixture.whenStable();
    fixture.detectChanges();
    expect((el.querySelector('textarea[aria-label=Antwort-Entwurf]') as HTMLTextAreaElement).value).toBe('Gruß an Benali und Okafor');
    http.verify();
  });
});
