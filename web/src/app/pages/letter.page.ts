import { Component, computed, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../core/api.service';
import { Explanation, Finding, Preview } from '../core/models';
import { problemMessage } from '../core/problem';
import { prepareUpload } from '../lib/image';
import { PLACEHOLDER, PLACEHOLDER_EXACT,redactManually, restoreExplanation } from '../lib/redaction';
import { ResultView } from '../result/result-view';

type Step = 'input' | 'preview' | 'result';
type TokenKind = 'mask' | 'server' | 'word' | 'plain';
interface Token { text: string; kind: TokenKind; value?: string }
// placeholders stay whole; everything else splits into words (letters/digits/marks) and separator runs
const TOKEN_SPLIT = new RegExp(String.raw`(${PLACEHOLDER.source}|(?:(?!${PLACEHOLDER.source})[^\p{L}\p{N}\p{M}])+)`, 'u');
const RING = 'focus-within:ring-2 focus-within:ring-brand-700 focus-within:ring-offset-2';
const MAX_UPLOAD = 10 * 1024 * 1024; // same limit as the server

@Component({
  selector: 'bk-letter',
  imports: [ResultView],
  template: `
    <h1 class="text-2xl font-bold">Neuer Brief</h1>
    @switch (step()) {
      @case ('input') {
        <div class="mt-6 grid gap-6 lg:grid-cols-2">
          <div class="card flex flex-col items-center justify-center gap-3 border-2 border-dashed p-8 text-center"
               (dragover)="$event.preventDefault()" (drop)="drop($event)">
            <span class="font-medium">Foto oder PDF hierher ziehen</span>
            <span class="text-sm text-slate-500">JPG, PNG, WebP, PDF · max. 10 MB</span>
            <div class="flex flex-wrap justify-center gap-2">
              <label class="btn btn-secondary cursor-pointer ${RING}">
                Foto aufnehmen
                <input type="file" class="sr-only" accept="image/jpeg,image/png,image/webp" capture="environment" (change)="pick($event)" />
              </label>
              <label class="btn btn-secondary cursor-pointer ${RING}">
                Datei wählen
                <input type="file" class="sr-only" accept="image/jpeg,image/png,image/webp,application/pdf" (change)="pick($event)" />
              </label>
            </div>
          </div>
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
          <p class="mt-1 text-sm text-slate-600">Gelb = geschwärzt. Klicke auf ein Wort, um es zusätzlich zu schwärzen; ein Klick auf einen Balken hebt die Schwärzung auf.</p>
          <p data-testid="preview-text" class="mt-4 rounded-lg bg-slate-50 p-4 text-sm leading-7 whitespace-pre-wrap">
            @for (tok of tokens(); track $index) {
              @if (tok.kind === 'mask') {
                <span data-mask [attr.data-placeholder]="tok.text" class="cursor-pointer rounded bg-amber-300 px-0.5 text-amber-900"
                      title="Klicken, um die Schwärzung aufzuheben" (click)="toggle(tok.value!)">█████</span>
              } @else if (tok.kind === 'server') {
                <span data-server [attr.data-placeholder]="tok.text" class="rounded bg-amber-300 px-0.5 text-amber-900">{{ tok.text }}</span>
              } @else if (tok.kind === 'word') {
                <span data-word class="cursor-pointer rounded px-0.5 hover:bg-amber-100" (click)="toggle(tok.text)">{{ tok.text }}</span>
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
        @if (recognized()) { <pre data-testid="recognized" class="mt-2 text-xs whitespace-pre-wrap">{{ recognized() }}</pre> }
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

  /** Single source of truth: exactly this text (and these findings) is shown and sent. */
  private readonly sent = computed(() => {
    const p = this.preview();
    return p ? redactManually(p.text, this.manual()) : { text: '', findings: [] as Finding[] };
  });

  protected readonly tokens = computed((): Token[] => {
    const { text, findings } = this.sent();
    const manualValues = new Map(findings.map((f) => [f.placeholder, f.value]));
    return text.split(TOKEN_SPLIT).filter((t) => t !== '').map((t): Token => {
      if (PLACEHOLDER_EXACT.test(t)) {
        const value = manualValues.get(t);
        return value === undefined ? { text: t, kind: 'server' } : { text: t, kind: 'mask', value };
      }
      return { text: t, kind: /[\p{L}\p{N}]/u.test(t) ? 'word' : 'plain' };
    });
  });

  protected toggle(word: string): void {
    this.manual.update((m) => (m.includes(word) ? m.filter((w) => w !== word) : [...m, word]));
  }

  protected pick(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = ''; // allow picking the same file again
    if (file) void this.upload(file);
  }

  protected drop(event: DragEvent): void {
    event.preventDefault();
    const file = event.dataTransfer?.files?.[0];
    if (file) void this.upload(file);
  }

  /** Photos are rotated and shrunk in the browser first; the size check applies to what would be sent. */
  private async upload(file: File): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    this.recognized.set('');
    const prepared = await prepareUpload(file);
    if (prepared.size > MAX_UPLOAD) {
      this.busy.set(false);
      this.error.set('Die Datei ist größer als 10 MB.');
      return;
    }
    await this.run(this.api.previewFile(prepared));
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
    const sent = this.sent();
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await firstValueFrom(this.api.explain(sent.text));
      const findings: Finding[] = [...p.findings, ...sent.findings, ...res.findings];
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
    this.recognized.set('');
  }
}
