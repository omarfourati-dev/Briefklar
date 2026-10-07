import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { ExplainResult, LoginResponse, Preview, Role, User } from './models';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);

  login(email: string, password: string) {
    return this.http.post<LoginResponse>('/api/auth/login', { email, password });
  }
  previewText(text: string) {
    const form = new FormData();
    form.append('text', text);
    return this.http.post<Preview>('/api/letters/preview', form);
  }
  previewFile(file: File) {
    const form = new FormData();
    form.append('file', file);
    return this.http.post<Preview>('/api/letters/preview', form);
  }
  explain(text: string) {
    return this.http.post<ExplainResult>('/api/letters/explain', { text });
  }
  users() {
    return this.http.get<User[]>('/api/users');
  }
  createUser(u: { email: string; name: string; password: string; role: Exclude<Role, 'demo'>; dailyLimit: number }) {
    return this.http.post<User>('/api/users', u);
  }
  setEnabled(id: string, enabled: boolean) {
    return this.http.patch<User>(`/api/users/${id}`, { enabled });
  }
  changePassword(currentPassword: string, newPassword: string) {
    return this.http.post<void>('/api/auth/password', { currentPassword, newPassword });
  }
}
