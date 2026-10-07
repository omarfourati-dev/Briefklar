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

  async function toPreview(text: string, previewText: string) {
    const fixture = TestBed.createComponent(LetterPage);
    const http = TestBed.inject(HttpTestingController);
    const el: HTMLElement = fixture.nativeElement;
    fixture.detectChanges();
    const textarea = el.querySelector('textarea') as HTMLTextAreaElement;
    textarea.value = text;
    textarea.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    (el.querySelector('[data-testid=preview]') as HTMLButtonElement).click();
    http.expectOne('/api/letters/preview').flush({ text: previewText, findings: [], inputKind: 'text' });
    await fixture.whenStable();
    fixture.detectChanges();
    const click = (pred: (s: Element) => boolean) => {
      ([...el.querySelectorAll('[data-word],[data-mask]')].find(pred) as HTMLElement).click();
      fixture.detectChanges();
    };
    // the preview text with masks mapped back to their placeholders
    const shown = () =>
      [...(el.querySelector('[data-testid=preview-text]') as HTMLElement).children]
        .map((c) => (c as HTMLElement).dataset['placeholder'] ?? c.textContent).join('');
    return { fixture, http, el, click, shown };
  }

  const LONG = 'Zahlung 5 EUR bis 15.11.2026 an Okafor, nicht an Okafors Bruder.';

  it('removes a one-character token from the request body', async () => {
    const { fixture, http, click, shown } = await toPreview(LONG, LONG);
    click((s) => s.textContent === '5');
    expect(shown()).toBe('Zahlung [MANUELL_1] EUR bis 15.11.2026 an Okafor, nicht an Okafors Bruder.');
    (fixture.nativeElement.querySelector('[data-testid=explain]') as HTMLButtonElement).click();
    const req = http.expectOne('/api/letters/explain');
    expect(req.request.body.text).toBe('Zahlung [MANUELL_1] EUR bis 15.11.2026 an Okafor, nicht an Okafors Bruder.');
  });

  it('matches whole words only and shows exactly what is sent', async () => {
    const { fixture, http, click, shown } = await toPreview(LONG, LONG);
    click((s) => s.textContent === 'Okafor');
    expect(shown()).toBe('Zahlung 5 EUR bis 15.11.2026 an [MANUELL_1], nicht an Okafors Bruder.');
    (fixture.nativeElement.querySelector('[data-testid=explain]') as HTMLButtonElement).click();
    const req = http.expectOne('/api/letters/explain');
    expect(req.request.body.text).toBe(shown());
    expect(req.request.body.text).toContain('Okafors');
  });

  it('un-hides a word when its mask is clicked again', async () => {
    const { fixture, http, el, click, shown } = await toPreview(LONG, LONG);
    click((s) => s.textContent === 'Okafor');
    expect(el.querySelector('[data-mask]')?.textContent).toBe('█████');
    click((s) => s.hasAttribute('data-mask'));
    expect(shown()).toBe(LONG);
    (el.querySelector('[data-testid=explain]') as HTMLButtonElement).click();
    const req = http.expectOne('/api/letters/explain');
    expect(req.request.body.text).toBe(LONG);
    req.flush({ explanation: { ...EXAMPLES[0].explanation, replyDraft: 'Hallo [MANUELL_1]' }, findings: [] });
    await fixture.whenStable();
    fixture.detectChanges();
    expect((el.querySelector('textarea[aria-label=Antwort-Entwurf]') as HTMLTextAreaElement).value).toBe('Hallo [MANUELL_1]');
  });

  it('shows the recognized text when the preview fails with 422', async () => {
    const fixture = TestBed.createComponent(LetterPage);
    const http = TestBed.inject(HttpTestingController);
    const el: HTMLElement = fixture.nativeElement;
    fixture.detectChanges();
    const textarea = el.querySelector('textarea') as HTMLTextAreaElement;
    textarea.value = LONG;
    textarea.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    (el.querySelector('[data-testid=preview]') as HTMLButtonElement).click();
    http.expectOne('/api/letters/preview').flush(
      { detail: 'Der Text ist zu kurz.', recognizedText: 'abc def' }, { status: 422, statusText: 'Unprocessable' });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Der Text ist zu kurz.');
    expect(el.querySelector('[data-testid=recognized]')?.textContent).toBe('abc def');
  });

  it('shows an explain error and stays on the preview', async () => {
    const { fixture, http, el } = await toPreview(LONG, LONG);
    (el.querySelector('[data-testid=explain]') as HTMLButtonElement).click();
    http.expectOne('/api/letters/explain').flush({ detail: 'KI nicht erreichbar.' }, { status: 502, statusText: 'Bad Gateway' });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role=alert]')?.textContent).toContain('KI nicht erreichbar.');
    expect(el.querySelector('[data-testid=explain]')).not.toBeNull();
    expect(el.querySelector('bk-result')).toBeNull();
  });

  async function hideAll(text: string, words: string[]) {
    const ctx = await toPreview(text, text);
    for (const w of words) ctx.click((s) => s.textContent === w && s.hasAttribute('data-word'));
    (ctx.el.querySelector('[data-testid=explain]') as HTMLButtonElement).click();
    const body: string = ctx.http.expectOne('/api/letters/explain').request.body.text;
    expect(body).toBe(ctx.shown());
    return body;
  }

  it('can hide every part of a hyphenated name', async () => {
    const body = await hideAll('Frau Okafor und Frau Müller-Okafor', ['Okafor', 'Müller']);
    expect(body).not.toContain('Okafor');
    expect(body).not.toContain('Müller');
    expect(body).toBe('Frau [MANUELL_1] und Frau [MANUELL_2]-[MANUELL_1]');
  });

  it('can hide words separated by a slash', async () => {
    const body = await hideAll('Okafor/Benali melden sich', ['Okafor', 'Benali']);
    expect(body).not.toContain('Benali');
    expect(body).not.toContain('Okafor');
  });

  it('handles Arabic-Indic digits and decomposed umlauts', async () => {
    const decomposed = 'Müller';
    const body = await hideAll(`Betrag ٥٠ EUR an ${decomposed} und Müllers`, ['٥٠', decomposed]);
    expect(body).not.toContain('٥٠');
    expect(body).not.toContain(decomposed);
    expect(body).toContain('Müllers');
  });

  it('offers a camera input and a file input', () => {
    const fixture = TestBed.createComponent(LetterPage);
    fixture.detectChanges();
    const inputs = [...(fixture.nativeElement as HTMLElement).querySelectorAll('input[type=file]')];
    expect(inputs.map((i) => i.getAttribute('capture'))).toEqual(['environment', null]);
    expect(inputs[0].getAttribute('accept')).toBe('image/jpeg,image/png,image/webp');
    expect(inputs[1].getAttribute('accept')).toBe('image/jpeg,image/png,image/webp,application/pdf');
  });
});
