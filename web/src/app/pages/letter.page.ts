import { Component, computed, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../core/api.service';
import { Explanation, Finding, Preview } from '../core/models';
import { problemMessage } from '../core/problem';
import { redactManually, restoreExplanation } from '../lib/redaction';
import { ResultView } from '../result/result-view';

type Step = 'input' | 'preview' | 'result';
const PLACEHOLDER = /^\[[A-Z_]+?_\d+\]$/;

@Component({
  selector: 'bk-letter',
  imports: [ResultView],
  template: `
    <h1 class="text-2xl font-bold">Neuer Brief</h1>
    @switch (step()) {
      @case ('input') {
        <div class="mt-6 grid gap-6 lg:grid-cols-2">
          <label class="card flex cursor-pointer flex-col items-center justify-center gap-2 border-2 border-dashed p-8 text-center"
                 (dragover)="$event.preventDefault()" (drop)="drop($event)">
            <span class="font-medium">Foto oder PDF hierher ziehen</span>
            <span class="text-sm text-slate-500">oder klicken · JPG, PNG, WebP, PDF · max. 10 MB</span>
            <input type="file" class="sr-only" accept="image/jpeg,image/png,image/webp,application/pdf" capture="environment" (change)="pick($event)" />
          </label>
          <div class="card p-5">
            <label class="label" for="text">… oder Text einfügen</label>
            <textarea id="text" class="input h-48" [value]="text()" (input)="text.set($any($event.target).value)"></textarea>
            <button type="button" data-testid="preview" class="btn btn-primary mt-3" [disabled]="busy() || text().trim().length < 20" (click)="previewText()">Weiter zur Vorschau</button>
          </div>
        </div>
        <p class="mt-4 text-sm text-slate-600">Die Texterkennung läuft auf unserem Server. Bevor etwas an die KI geht, siehst du, was geschwärzt wird.</p>
      }
      @case ('preview') {
        <div class="card mt-6 p-5">
          <h2 class="font-semibold">Vorschau: das bekommt die KI</h2>
          <p class="mt-1 text-sm text-slate-600">Gelb = geschwärzt. Klicke auf ein Wort, um es zusätzlich zu schwärzen; nochmal klicken hebt es auf.</p>
          <p class="mt-4 rounded-lg bg-slate-50 p-4 text-sm leading-7 whitespace-pre-wrap">
            @for (tok of tokens(); track $index) {
              @if (tok.word) {
                <span data-word class="cursor-pointer rounded px-0.5"
                      [class]="tok.hidden ? 'bg-amber-300 text-amber-900' : 'hover:bg-amber-100'"
                      (click)="toggle(tok.text)">{{ tok.hidden && !tok.placeholder ? '█████' : tok.text }}</span>
              } @else {<span>{{ tok.text }}</span>}
            }
          </p>
          <div class="mt-4 flex flex-wrap gap-2">
            <button type="button" data-testid="explain" class="btn btn-primary" [disabled]="busy()" (click)="explain()">{{ busy() ? 'Wird erklärt …' : 'Erklären lassen' }}</button>
            <button type="button" class="btn btn-secondary" [disabled]="busy()" (click)="reset()">Anderen Brief</button>
          </div>
        </div>
      }
      @case ('result') {
        <div class="mt-6"><bk-result [explanation]="result()!" /></div>
        <button type="button" class="btn btn-secondary mt-4" (click)="reset()">Nächster Brief</button>
      }
    }
    @if (busy() && step() === 'input') { <p class="mt-4 text-sm text-slate-600">Text wird erkannt …</p> }
    @if (error()) {
      <div class="mt-4 rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800" role="alert">
        {{ error() }}
        @if (recognized()) { <pre class="mt-2 text-xs whitespace-pre-wrap">{{ recognized() }}</pre> }
      </div>
    }
  `,
})
export class LetterPage {
  private readonly api = inject(ApiService);
  protected readonly step = signal<Step>('input');
  protected readonly text = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly recognized = signal('');
  private readonly preview = signal<Preview | null>(null);
  private readonly manual = signal<string[]>([]);
  protected readonly result = signal<Explanation | null>(null);

  protected readonly tokens = computed(() => {
    const p = this.preview();
    if (!p) return [];
    const hidden = new Set(this.manual());
    return p.text.split(/(\s+|[.,;:!?()])/).filter((t) => t !== '').map((t) => {
      const placeholder = PLACEHOLDER.test(t);
      const word = /\p{L}|\d/u.test(t);
      return { text: t, word, placeholder, hidden: placeholder || hidden.has(t) };
    });
  });

  protected toggle(word: string): void {
    if (PLACEHOLDER.test(word)) return;
    this.manual.update((m) => (m.includes(word) ? m.filter((w) => w !== word) : [...m, word]));
  }

  protected pick(event: Event): void {
    const file = (event.target as HTMLInputElement).files?.[0];
    if (file) void this.run(this.api.previewFile(file));
  }

  protected drop(event: DragEvent): void {
    event.preventDefault();
    const file = event.dataTransfer?.files?.[0];
    if (file) void this.run(this.api.previewFile(file));
  }

  protected previewText(): void {
    void this.run(this.api.previewText(this.text()));
  }

  private async run(request: ReturnType<ApiService['previewText']>): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    this.recognized.set('');
    try {
      this.preview.set(await firstValueFrom(request));
      this.manual.set([]);
      this.step.set('preview');
    } catch (err) {
      this.error.set(problemMessage(err, 'Der Brief konnte nicht gelesen werden.'));
      if (err instanceof HttpErrorResponse && err.status === 422) this.recognized.set(err.error?.recognizedText ?? '');
    } finally {
      this.busy.set(false);
    }
  }

  protected async explain(): Promise<void> {
    const p = this.preview()!;
    const manual = redactManually(p.text, this.manual());
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await firstValueFrom(this.api.explain(manual.text));
      const findings: Finding[] = [...p.findings, ...manual.findings, ...res.findings];
      this.result.set(restoreExplanation(res.explanation, findings));
      this.step.set('result');
    } catch (err) {
      this.error.set(problemMessage(err, 'Die Erklärung ist gerade nicht möglich.'));
    } finally {
      this.busy.set(false);
    }
  }

  protected reset(): void {
    this.step.set('input');
    this.preview.set(null);
    this.result.set(null);
    this.text.set('');
    this.error.set('');
  }
}
