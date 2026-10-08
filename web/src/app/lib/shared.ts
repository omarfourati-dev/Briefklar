import { InjectionToken } from '@angular/core';

/**
 * Android "Share → Briefklar": the service worker (public/sw.js) receives the POST to /app/share-target and parks
 * the photo, PDF or text in this cache for a moment. The letter page picks it up once – after a login, too – and
 * deletes it right away, so nothing from a letter stays on the device.
 */
export const SHARE_CACHE = 'briefklar-share';
export const SHARE_KEY = '/app/__shared__';
const MAX_AGE_MS = 10 * 60_000;

export type Shared = { kind: 'file'; file: File } | { kind: 'text'; text: string };

export async function takeShared(storage: CacheStorage | undefined = globalThis.caches, now = Date.now()): Promise<Shared | null> {
  if (!storage) return null;
  const cache = await storage.open(SHARE_CACHE);
  const res = await cache.match(SHARE_KEY);
  if (!res) return null;
  await cache.delete(SHARE_KEY);
  const at = Number(res.headers.get('X-Shared-At'));
  if (!at || now - at > MAX_AGE_MS) return null;
  if (res.headers.get('X-Kind') === 'text') return { kind: 'text', text: await res.text() };
  const type = res.headers.get('Content-Type') ?? '';
  const name = decodeURIComponent(res.headers.get('X-Name') ?? 'geteilt');
  return { kind: 'file', file: new File([await res.blob()], name, { type }) };
}

/** Injectable so the letter page can be tested without a real Cache API. */
export const SHARED_INBOX = new InjectionToken<() => Promise<Shared | null>>('SHARED_INBOX', {
  providedIn: 'root',
  factory: () => () => takeShared().catch(() => null),
});
