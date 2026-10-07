export type Role = 'user' | 'admin' | 'demo';
export interface Me { email: string; name: string; role: Role; }
export interface LoginResponse { token: string; expiresAt: string; user: Me; }
export interface Finding { placeholder: string; kind: string; value: string; }
export interface Texts { de: string; en: string; fr: string; ar: string; }
export type Lang = keyof Texts;
export type Urgency = 'red' | 'yellow' | 'green';
export interface Explanation {
  authority: string;
  letterType: string;
  deadline: string | null;
  deadlineText: string;
  urgency: Urgency;
  summary: Texts;
  actions: Texts[];
  replyDraft: string;
  missingInfo: string[];
}
export interface Preview { text: string; findings: Finding[]; inputKind: 'text' | 'pdf' | 'image'; }
export interface ExplainResult { explanation: Explanation; findings: Finding[]; }
export interface User {
  id: string; email: string; name: string; role: Role;
  enabled: boolean; dailyLimit: number; usedToday: number; createdAt: string;
}
