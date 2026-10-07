import { TestBed } from '@angular/core/testing';
import { ResultView } from './result-view';
import { EXAMPLES } from '../lib/examples';

describe('ResultView', () => {
  function render() {
    const fixture = TestBed.createComponent(ResultView);
    fixture.componentRef.setInput('explanation', EXAMPLES[0].explanation);
    fixture.componentRef.setInput('today', new Date('2026-10-07T12:00:00'));
    fixture.detectChanges();
    return fixture;
  }

  it('shows German by default and switches to Arabic right-to-left', () => {
    const fixture = render();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('[data-testid=summary]')!.textContent).toContain('Aufenthaltserlaubnis');
    (el.querySelector('button[data-lang=ar]') as HTMLButtonElement).click();
    fixture.detectChanges();
    const summary = el.querySelector('[data-testid=summary]')!;
    expect(summary.getAttribute('dir')).toBe('rtl');
    expect(summary.textContent).toContain('إقامتك');
  });

  it('shows the deadline, the disclaimer and the reply draft', () => {
    const el: HTMLElement = render().nativeElement;
    expect(el.textContent).toContain('15.11.2026');
    expect(el.textContent).toContain('Keine Rechtsberatung');
    expect((el.querySelector('textarea') as HTMLTextAreaElement).value).toContain('Max Beispiel');
  });

  it('hides the deadline block and the calendar button for an invalid deadline', () => {
    const fixture = TestBed.createComponent(ResultView);
    fixture.componentRef.setInput('explanation', { ...EXAMPLES[0].explanation, deadline: '2026-02-31' });
    fixture.componentRef.setInput('today', new Date('2026-10-07T12:00:00'));
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).not.toContain('In den Kalender');
    expect(el.textContent).not.toContain(EXAMPLES[0].explanation.deadlineText);
  });
});
