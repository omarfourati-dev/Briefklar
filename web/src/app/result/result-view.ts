import { Component, computed, input, signal } from '@angular/core';
import { Explanation, Lang } from '../core/models';
import { buildIcs } from '../lib/ics';
import { daysUntil, isValidDate, urgencyFor } from '../lib/urgency';

const LANGS: { id: Lang; label: string }[] = [
  { id: 'de', label: 'Deutsch' }, { id: 'en', label: 'English' }, { id: 'fr', label: 'Français' }, { id: 'ar', label: 'العربية' },
];

@Component({
  selector: 'bk-result',
  template: `
    @let e = explanation();
    <div class="space-y-5">
      <div class="card flex flex-wrap items-start justify-between gap-4 p-5">
        <div>
          <p class="text-xs font-semibold tracking-wide text-slate-500 uppercase">{{ e.letterType }}</p>
          <h2 class="mt-1 text-xl font-bold text-slate-900">{{ e.authority }}</h2>
        </div>
        <span class="rounded-full px-3 py-1 text-sm font-semibold" [class]="badgeClass()">{{ badgeText() }}</span>
      </div>

      @if (e.deadline && validDeadline()) {
        <div class="card flex flex-wrap items-center justify-between gap-3 p-5">
          <div>
            <p class="text-sm text-slate-500">Frist</p>
            <p class="text-lg font-semibold">{{ deadlineLabel() }}</p>
            <p class="text-sm text-slate-600">{{ e.deadlineText }}</p>
          </div>
          <button type="button" class="btn btn-primary" (click)="downloadIcs()">In den Kalender (.ics)</button>
        </div>
      }

      <div class="card p-5">
        <div class="flex flex-wrap gap-2" role="tablist" aria-label="Sprache">
          @for (l of langs; track l.id) {
            <button type="button" role="tab" class="btn" [attr.data-lang]="l.id" [attr.aria-selected]="lang() === l.id"
                    [class]="lang() === l.id ? 'btn-primary' : 'btn-secondary'" (click)="lang.set(l.id)">{{ l.label }}</button>
          }
        </div>
        <p data-testid="summary" class="mt-4 leading-relaxed" [attr.dir]="lang() === 'ar' ? 'rtl' : 'ltr'" [attr.lang]="lang()">
          {{ e.summary[lang()] }}
        </p>
      </div>

      @if (e.actions.length) {
        <div class="card p-5">
          <h3 class="font-semibold">Was ist zu tun?</h3>
          <ul class="mt-3 space-y-2" [attr.dir]="lang() === 'ar' ? 'rtl' : 'ltr'">
            @for (a of e.actions; track $index) {
              <li><label class="flex items-start gap-2"><input type="checkbox" class="mt-1" /> <span>{{ a[lang()] }}</span></label></li>
            }
          </ul>
        </div>
      }

      <div class="card p-5">
        <div class="flex items-center justify-between gap-2">
          <h3 class="font-semibold">Antwort-Entwurf</h3>
          <button type="button" class="btn btn-secondary" (click)="copy()">{{ copied() ? 'Kopiert ✓' : 'Kopieren' }}</button>
        </div>
        <textarea class="input mt-3 h-56 font-mono text-xs" readonly [value]="e.replyDraft" aria-label="Antwort-Entwurf"></textarea>
      </div>

      @if (e.missingInfo.length) {
        <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
          <p class="font-semibold">Unklar im Brief</p>
          <ul class="mt-1 list-disc pl-5">@for (m of e.missingInfo; track $index) { <li>{{ m }}</li> }</ul>
        </div>
      }

      <p class="text-xs text-slate-500">Keine Rechtsberatung. Im Zweifel bei der Behörde nachfragen.</p>
    </div>
  `,
})
export class ResultView {
  readonly explanation = input.required<Explanation>();
  readonly today = input<Date>(new Date());
  protected readonly langs = LANGS;
  protected readonly lang = signal<Lang>('de');
  protected readonly copied = signal(false);

  protected readonly urgency = computed(() => urgencyFor(this.explanation().deadline, this.explanation().urgency, this.today()));
  protected readonly validDeadline = computed(() => {
    const d = this.explanation().deadline;
    return !!d && isValidDate(d);
  });
  protected readonly badgeText = computed(() =>
    ({ red: 'Dringend', yellow: 'Frist beachten', green: 'Zur Information' })[this.urgency()]);
  protected readonly badgeClass = computed(() =>
    ({ red: 'bg-rose-100 text-rose-800', yellow: 'bg-amber-100 text-amber-800', green: 'bg-emerald-100 text-emerald-800' })[this.urgency()]);
  protected readonly deadlineLabel = computed(() => {
    const d = this.explanation().deadline!;
    const [y, m, day] = d.split('-');
    const left = daysUntil(d, this.today());
    const rest = left < 0 ? `seit ${-left} Tagen vorbei` : left === 0 ? 'heute' : `noch ${left} Tage`;
    return `${day}.${m}.${y} – ${rest}`;
  });

  protected downloadIcs(): void {
    const e = this.explanation();
    const ics = buildIcs({ title: `Frist: ${e.authority}`, date: e.deadline!, description: `${e.letterType}\n${e.deadlineText}` });
    const url = URL.createObjectURL(new Blob([ics], { type: 'text/calendar;charset=utf-8' }));
    const a = Object.assign(document.createElement('a'), { href: url, download: 'briefklar-frist.ics' });
    a.click();
    URL.revokeObjectURL(url);
  }

  protected async copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(this.explanation().replyDraft);
      this.copied.set(true);
      setTimeout(() => this.copied.set(false), 2000);
    } catch { /* clipboard blocked: the textarea stays selectable */ }
  }
}
