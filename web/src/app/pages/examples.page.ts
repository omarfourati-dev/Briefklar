import { Component, signal } from '@angular/core';
import { EXAMPLES, Example } from '../lib/examples';
import { ResultView } from '../result/result-view';

@Component({
  selector: 'bk-examples',
  imports: [ResultView],
  template: `
    <h1 class="text-2xl font-bold">Beispielbriefe</h1>
    <p class="mt-1 text-slate-600">Erfundene Briefe mit fertiger Erklärung – so sieht ein Ergebnis aus.</p>
    <div class="mt-6 grid gap-3 md:grid-cols-3">
      @for (ex of examples; track ex.id) {
        <button type="button" class="card p-4 text-left hover:border-brand-600" [class.border-brand-600]="selected()?.id === ex.id" (click)="selected.set(ex)">
          <span class="font-medium">{{ ex.title }}</span>
        </button>
      }
    </div>
    @if (selected(); as ex) {
      <div class="mt-6 grid gap-6 lg:grid-cols-2">
        <section class="card p-5">
          <h2 class="font-semibold">Der Brief</h2>
          <pre class="mt-3 text-sm whitespace-pre-wrap text-slate-700">{{ ex.letter }}</pre>
        </section>
        <bk-result [explanation]="ex.explanation" />
      </div>
    }
  `,
})
export class ExamplesPage {
  protected readonly examples = EXAMPLES;
  protected readonly selected = signal<Example | null>(null);
}
