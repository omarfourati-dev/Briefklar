import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../core/api.service';
import { AuthStore } from '../core/auth.store';
import { problemMessage } from '../core/problem';

const DEMO = { email: 'demo@briefklar.app', password: 'demo-briefklar' };

@Component({
  selector: 'bk-login',
  imports: [ReactiveFormsModule],
  template: `
    <div class="mx-auto mt-16 max-w-sm px-4">
      <a href="/" class="text-xl font-bold text-slate-900">Briefklar</a>
      <form class="card mt-6 space-y-4 p-6" [formGroup]="form" (ngSubmit)="submit()">
        <h1 class="text-lg font-semibold">Anmelden</h1>
        <div><label class="label" for="email">E-Mail</label><input id="email" class="input" type="email" formControlName="email" autocomplete="username" /></div>
        <div><label class="label" for="password">Passwort</label><input id="password" class="input" type="password" formControlName="password" autocomplete="current-password" /></div>
        @if (error()) { <p class="text-sm text-rose-600" role="alert">{{ error() }}</p> }
        <button type="submit" class="btn btn-primary w-full" [disabled]="busy() || form.invalid">Anmelden</button>
      </form>
      <div class="card mt-4 p-4 text-sm text-slate-600">
        <p class="font-medium text-slate-800">Demo ansehen</p>
        <p class="mt-1">Drei Beispielbriefe mit fertiger Erklärung, ohne eigenes Konto.</p>
        <button type="button" class="btn btn-secondary mt-3 w-full" [disabled]="busy()" (click)="demo()">Als Demo anmelden</button>
        <p class="mt-3">Kein Konto? <a class="font-medium text-brand-700" href="mailto:info@omarfourati.de?subject=Briefklar%20%E2%80%93%20Zugang%20anfragen">Zugang anfragen</a></p>
      </div>
    </div>
  `,
})
export class LoginPage {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly form = inject(FormBuilder).nonNullable.group({
    email: ['', [Validators.required, Validators.email]],
    password: ['', Validators.required],
  });

  protected demo(): void {
    this.form.setValue(DEMO);
    void this.submit();
  }

  protected async submit(): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    try {
      const { email, password } = this.form.getRawValue();
      this.auth.set(await firstValueFrom(this.api.login(email, password)));
      await this.router.navigateByUrl('/');
    } catch (err) {
      this.error.set(problemMessage(err, 'Anmeldung fehlgeschlagen.'));
    } finally {
      this.busy.set(false);
    }
  }
}
