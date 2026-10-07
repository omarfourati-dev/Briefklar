import { DatePipe } from '@angular/common';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../core/api.service';
import { AuthStore } from '../core/auth.store';
import { User } from '../core/models';
import { problemMessage } from '../core/problem';

@Component({
  selector: 'bk-admin',
  imports: [ReactiveFormsModule, DatePipe],
  template: `
    <div class="grid gap-6 lg:grid-cols-3">
      <section class="card overflow-x-auto lg:col-span-2">
        <h1 class="px-5 pt-5 text-xl font-bold">Benutzer</h1>
        <table class="mt-3 w-full min-w-[560px] text-sm">
          <thead class="border-y border-slate-200 bg-slate-50 text-left text-xs text-slate-500 uppercase">
            <tr><th class="px-5 py-2">Name</th><th class="px-5 py-2">E-Mail</th><th class="px-5 py-2">Rolle</th><th class="px-5 py-2">Heute</th><th class="px-5 py-2">Seit</th><th class="px-5 py-2">Status</th></tr>
          </thead>
          <tbody>
            @for (u of users(); track u.id) {
              <tr class="border-b border-slate-100" [class.text-slate-400]="!u.enabled">
                <td class="px-5 py-2 font-medium">{{ u.name }}</td>
                <td class="px-5 py-2">{{ u.email }}</td>
                <td class="px-5 py-2">{{ u.role }}</td>
                <td class="px-5 py-2">{{ u.usedToday }} / {{ u.dailyLimit }}</td>
                <td class="px-5 py-2">{{ u.createdAt | date: 'dd.MM.yyyy' }}</td>
                <td class="px-5 py-2">
                  @if (u.email === auth.user()?.email) { <span class="text-xs">Du</span> }
                  @else { <button type="button" class="text-xs font-medium hover:underline" [class]="u.enabled ? 'text-rose-600' : 'text-brand-700'" (click)="toggle(u)">{{ u.enabled ? 'Sperren' : 'Entsperren' }}</button> }
                </td>
              </tr>
            }
          </tbody>
        </table>
      </section>
      <form class="card space-y-3 p-5" [formGroup]="form" (ngSubmit)="create()">
        <h2 class="font-semibold">Neuer Benutzer</h2>
        <div><label class="label" for="name">Name</label><input id="name" class="input" formControlName="name" /></div>
        <div><label class="label" for="mail">E-Mail</label><input id="mail" type="email" class="input" formControlName="email" /></div>
        <div><label class="label" for="pw">Startpasswort (mind. 12 Zeichen)</label><input id="pw" type="password" class="input" formControlName="password" autocomplete="new-password" /></div>
        <div><label class="label" for="role">Rolle</label>
          <select id="role" class="input" formControlName="role"><option value="user">Benutzer</option><option value="admin">Admin</option></select></div>
        <div><label class="label" for="limit">Tageslimit</label><input id="limit" type="number" min="0" max="1000" class="input" formControlName="dailyLimit" /></div>
        @if (message()) { <p class="text-sm" role="status">{{ message() }}</p> }
        <button type="submit" class="btn btn-primary w-full" [disabled]="form.invalid">Anlegen</button>
      </form>
    </div>
  `,
})
export class AdminPage implements OnInit {
  private readonly api = inject(ApiService);
  protected readonly auth = inject(AuthStore);
  protected readonly users = signal<User[]>([]);
  protected readonly message = signal('');
  protected readonly form = inject(FormBuilder).nonNullable.group({
    name: ['', Validators.required],
    email: ['', [Validators.required, Validators.email]],
    password: ['', [Validators.required, Validators.minLength(12)]],
    role: ['user' as 'user' | 'admin'],
    dailyLimit: [20, [Validators.min(0), Validators.max(1000)]],
  });

  ngOnInit(): void { void this.load(); }

  private async load(): Promise<void> {
    this.users.set(await firstValueFrom(this.api.users()));
  }

  protected async create(): Promise<void> {
    try {
      const u = await firstValueFrom(this.api.createUser(this.form.getRawValue()));
      this.message.set(`${u.name} wurde angelegt.`);
      this.form.reset({ role: 'user', dailyLimit: 20 });
      await this.load();
    } catch (err) {
      this.message.set(problemMessage(err, 'Benutzer konnte nicht angelegt werden.'));
    }
  }

  protected async toggle(u: User): Promise<void> {
    try {
      await firstValueFrom(this.api.setEnabled(u.id, !u.enabled));
      await this.load();
    } catch (err) {
      this.message.set(problemMessage(err, 'Status konnte nicht geändert werden.'));
    }
  }
}
