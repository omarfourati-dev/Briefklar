import { describe, expect, it } from 'vitest';
import { SHARE_CACHE, SHARE_KEY, takeShared } from './shared';

/** Minimal CacheStorage with one cache, enough for takeShared. */
function fakeCaches(entry?: Response) {
  const store = new Map<string, Response>(entry ? [[SHARE_KEY, entry]] : []);
  const opened: string[] = [];
  const cache = {
    match: async (key: string) => store.get(key),
    delete: async (key: string) => store.delete(key),
  };
  const storage = {
    open: async (name: string) => (opened.push(name), cache),
  } as unknown as CacheStorage;
  return { storage, store, opened };
}

const shared = (body: BodyInit, headers: Record<string, string>) => new Response(body, { headers });
const NOW = 1_800_000_000_000;

describe('takeShared', () => {
  it('returns a shared file with name and type and removes it from the cache', async () => {
    const { storage, store, opened } = fakeCaches(shared(new TextEncoder().encode('%PDF-1.4'), {
      'Content-Type': 'application/pdf', 'X-Kind': 'file', 'X-Name': encodeURIComponent('Bescheid März.pdf'), 'X-Shared-At': String(NOW - 1000),
    }));
    const got = await takeShared(storage, NOW);
    expect(opened).toEqual([SHARE_CACHE]);
    expect(got?.kind).toBe('file');
    if (got?.kind !== 'file') return;
    expect(got.file.name).toBe('Bescheid März.pdf');
    expect(got.file.type).toBe('application/pdf');
    expect(await got.file.text()).toBe('%PDF-1.4');
    expect(store.size).toBe(0);
  });

  it('returns shared text', async () => {
    const { storage } = fakeCaches(shared('Sehr geehrter Herr …', {
      'Content-Type': 'text/plain; charset=utf-8', 'X-Kind': 'text', 'X-Shared-At': String(NOW),
    }));
    expect(await takeShared(storage, NOW)).toEqual({ kind: 'text', text: 'Sehr geehrter Herr …' });
  });

  it('drops a share older than ten minutes without using it', async () => {
    const { storage, store } = fakeCaches(shared('alt', { 'X-Kind': 'text', 'X-Shared-At': String(NOW - 11 * 60_000) }));
    expect(await takeShared(storage, NOW)).toBeNull();
    expect(store.size).toBe(0);
  });

  it('drops an entry without a timestamp', async () => {
    const { storage, store } = fakeCaches(shared('x', { 'X-Kind': 'text' }));
    expect(await takeShared(storage, NOW)).toBeNull();
    expect(store.size).toBe(0);
  });

  it('returns null when nothing was shared or the browser has no Cache API', async () => {
    expect(await takeShared(fakeCaches().storage, NOW)).toBeNull();
    expect(await takeShared(undefined, NOW)).toBeNull();
  });
});
