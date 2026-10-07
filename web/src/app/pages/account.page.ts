import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../core/api.service';
import { problemMessage } from '../core/problem';

@Component({
  selector: 'bk-account',
  imports: [ReactiveFormsModule],
  template: `
    <h1 class="text-2xl font-bold">Konto</h1>
    <form class="card mt-6 max-w-md space-y-4 p-6" [formGroup]="form" (ngSubmit)="submit()">
      <h2 class="font-semibold">Passwort ändern</h2>
      <div><label class="label" for="current">Aktuelles Passwort</label><input id="current" type="password" class="input" formControlName="current" autocomplete="current-password" /></div>
      <div><label class="label" for="next">Neues Passwort (mind. 12 Zeichen)</label><input id="next" type="password" class="input" formControlName="next" autocomplete="new-password" /></div>
      @if (message()) { <p class="text-sm" [class]="ok() ? 'text-emerald-700' : 'text-rose-600'" role="status">{{ message() }}</p> }
      <button type="submit" class="btn btn-primary" [disabled]="form.invalid || busy()">Speichern</button>
    </form>
  `,
})
export class AccountPage {
  private readonly api = inject(ApiService);
  protected readonly busy = signal(false);
  protected readonly ok = signal(false);
  protected readonly message = signal('');
  protected readonly form = inject(FormBuilder).nonNullable.group({
    current: ['', Validators.required],
    next: ['', [Validators.required, Validators.minLength(12)]],
  });

  protected async submit(): Promise<void> {
    this.busy.set(true);
    try {
      const { current, next } = this.form.getRawValue();
      await firstValueFrom(this.api.changePassword(current, next));
      this.ok.set(true);
      this.message.set('Passwort geändert.');
      this.form.reset();
    } catch (err) {
      this.ok.set(false);
      this.message.set(problemMessage(err, 'Passwort konnte nicht geändert werden.'));
    } finally {
      this.busy.set(false);
    }
  }
}
