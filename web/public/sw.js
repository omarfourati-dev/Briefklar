// Briefklar service worker – scope /app/.
// Caches only the app shell (HTML, hashed JS/CSS, icons) so the installed app starts fast.
// Never cached: /api (letters, explanations, tokens), anything outside /app/, anything but GET.
// scripts/stamp-sw.mjs fills in BUILD and ASSETS at build time: every deploy is a new worker ("Neu laden"),
// and all JS/CSS – including lazy-loaded pages – is cached on install.
const BUILD = '__BUILD__';
const ASSETS = [/*__ASSETS__*/];
const SHELL = 'briefklar-shell-' + BUILD;
const SHARE = 'briefklar-share'; // see src/app/lib/shared.ts
const SCOPE = new URL('./', self.location).pathname; // "/app/"
const HASHED = /-[A-Za-z0-9_-]{8}\.(js|css)$/; // same rule as internal/server/static.go
const ICONS = ['icons/icon-192.png', 'icons/icon-512.png', 'icons/maskable-512.png'].map((p) => SCOPE + p);

self.addEventListener('install', (event) => {
  event.waitUntil(precache());
});

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    for (const name of await caches.keys()) {
      if (name.startsWith('briefklar-shell-') && name !== SHELL) await caches.delete(name);
    }
    await self.clients.claim();
  })());
});

self.addEventListener('message', (event) => {
  if (event.data === 'skip-waiting') self.skipWaiting();
});

self.addEventListener('fetch', (event) => {
  const req = event.request;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin || !url.pathname.startsWith(SCOPE)) return; // /api, /healthz, landing
  if (req.method === 'POST' && url.pathname === SCOPE + 'share-target') {
    event.respondWith(receiveShare(req));
    return;
  }
  if (req.method !== 'GET') return;
  if (req.mode === 'navigate') event.respondWith(networkFirst(req));
  else if (HASHED.test(url.pathname) || ICONS.includes(url.pathname)) event.respondWith(cacheFirst(req));
});

/** The shell: index.html plus every JS/CSS file of this build, so the app also opens offline. */
async function precache() {
  const cache = await caches.open(SHELL);
  await cache.addAll([SCOPE, ...ASSETS.map((f) => SCOPE + f), ...ICONS]);
}

/** Pages: always the newest version when online; offline the cached shell (the app shows "Keine Verbindung"). */
async function networkFirst(req) {
  const cache = await caches.open(SHELL);
  try {
    const res = await fetch(req);
    if (res.ok && (res.headers.get('Content-Type') || '').includes('text/html')) await cache.put(SCOPE, res.clone());
    return res;
  } catch (err) {
    const cached = await cache.match(SCOPE);
    if (cached) return cached;
    throw err;
  }
}

/** Hashed files never change: cache on first use. */
async function cacheFirst(req) {
  const cache = await caches.open(SHELL);
  const cached = await cache.match(req);
  if (cached) return cached;
  const res = await fetch(req);
  if (res.ok) await cache.put(req, res.clone());
  return res;
}

/** Android share sheet → park the photo/PDF/text for the letter page, which takes and deletes it at once. */
async function receiveShare(req) {
  const form = await req.formData();
  const file = form.get('file');
  const text = [form.get('title'), form.get('text'), form.get('url')].filter((v) => typeof v === 'string' && v.trim()).join('\n');
  const cache = await caches.open(SHARE);
  const at = String(Date.now());
  if (file instanceof File && file.size > 0) {
    await cache.put(SCOPE + '__shared__', new Response(file, {
      headers: { 'Content-Type': file.type, 'X-Kind': 'file', 'X-Name': encodeURIComponent(file.name), 'X-Shared-At': at },
    }));
  } else if (text) {
    await cache.put(SCOPE + '__shared__', new Response(text, {
      headers: { 'Content-Type': 'text/plain; charset=utf-8', 'X-Kind': 'text', 'X-Shared-At': at },
    }));
  }
  return Response.redirect(SCOPE + 'neu', 303);
}
