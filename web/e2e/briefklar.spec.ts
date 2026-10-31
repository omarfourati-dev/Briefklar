import { APIRequestContext, expect, Page, test } from '@playwright/test';

const ADMIN_EMAIL = process.env['E2E_ADMIN_EMAIL'] ?? 'admin@briefklar.test';
const ADMIN_PASSWORD = process.env['E2E_ADMIN_PASSWORD'] ?? 'local-admin-password-123';
const run = Date.now().toString(36);
// The app runs on its own origin app.<site> (APP_ORIGIN), away from the AdSense scripts of the landing page.
// Chromium resolves *.localhost itself; Node may not, so API calls from the test go to BASE with the app's Host header.
const BASE = process.env['E2E_BASE_URL'] ?? 'http://localhost:8080';
const APP_HOST = `app.${new URL(BASE).host}`;
const APP = `${new URL(BASE).protocol}//${APP_HOST}`;
const ON_APP = { headers: { host: APP_HOST } };
const DISCLAIMER = 'Keine Rechtsberatung. Im Zweifel bei der Behörde nachfragen.';

test('landing page explains Briefklar and leads to the demo', async ({ page, request }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Briefklar/);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Was will das Amt');
  await expect(page.getByText(DISCLAIMER).first()).toBeVisible();
  JSON.parse((await page.locator('script[type="application/ld+json"]').first().textContent()) ?? '');
  await page.getByRole('link', { name: 'Demo ansehen' }).first().click();
  await expect(page.getByRole('button', { name: 'Als Demo anmelden' })).toBeVisible();
  await expect(page.getByText(DISCLAIMER).first()).toBeVisible();
  expect((await request.get('/llms.txt')).ok()).toBeTruthy();
  expect((await request.get('/healthz')).ok()).toBeTruthy();
  expect((await request.get('/metrics')).ok()).toBeTruthy(); // direct; Caddy blocks it in production
});

test('ads only on the landing page – the app keeps its strict CSP and loads no Google code', async ({ page, request }) => {
  const landing = await request.get('/');
  expect(await landing.text()).toContain('pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-');
  expect(landing.headers()['content-security-policy']).toContain("script-src 'self' 'unsafe-inline' https:");
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Datenschutzeinstellungen' })).toBeVisible();

  const app = await request.get(`${BASE}/app/login`, ON_APP);
  expect(await app.text()).not.toContain('googlesyndication');
  expect(app.headers()['content-security-policy']).not.toContain('https:');
  expect(app.headers()['referrer-policy']).toBe('no-referrer');
});

test('the app runs on its own origin – no login and no session on the page with ads', async ({ page, request }) => {
  const moved = await request.get(`${BASE}/app/neu?x=1`, { maxRedirects: 0 });
  expect(moved.status()).toBe(301);
  expect(moved.headers()['location']).toBe(`${APP}/app/neu?x=1`);
  expect((await request.post(`${BASE}/api/auth/login`, { data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD } })).status()).toBe(404);
  const appRoot = await request.get(`${BASE}/`, { ...ON_APP, maxRedirects: 0 });
  expect(appRoot.status()).toBe(302);
  expect(appRoot.headers()['content-security-policy']).not.toContain('https:'); // never the AdSense CSP on the app

  // logged in on the app, then back to the landing page in the same tab: the session is not there
  await page.goto('/app/login');
  await page.getByRole('button', { name: 'Als Demo anmelden' }).click();
  await expect(page.getByRole('heading', { name: 'Beispielbriefe' })).toBeVisible();
  expect(await page.evaluate(() => sessionStorage.getItem('briefklar.session'))).toContain('token');
  await page.goto(`${BASE}/`);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Was will das Amt');
  expect(await page.evaluate(() => [sessionStorage.getItem('briefklar.session'), localStorage.getItem('briefklar.session')]))
    .toEqual([null, null]);
  // an app session left on the site from before the move is dropped before the ads load
  await page.evaluate(() => sessionStorage.setItem('briefklar.session', '{"token":"alt"}'));
  await page.reload();
  expect(await page.evaluate(() => sessionStorage.getItem('briefklar.session'))).toBeNull();
});

test('login page shows the demo credentials and the Angular app is styled', async ({ page }) => {
  await page.goto('/app/login');
  await expect(page).toHaveURL(`${APP}/app/login`); // the site sends the app to its own origin
  await expect(page.getByRole('link', { name: 'Briefklar' })).toHaveAttribute('href', `${BASE}/`); // home = the site
  await expect(page.getByText('demo@briefklar.app')).toBeVisible();
  await expect(page.getByText('demo-briefklar')).toBeVisible();
  // CSP forbids inline handlers (inlineCritical=false): the stylesheet must really be applied
  const background = await page.locator('.btn-primary').first().evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(background).not.toBe('rgba(0, 0, 0, 0)');
  expect(background).not.toBe('transparent');
});

test('demo sees examples, switches to Arabic and downloads the deadline', async ({ page }) => {
  await page.goto('/app/login');
  await page.getByRole('button', { name: 'Als Demo anmelden' }).click();
  await expect(page.getByRole('heading', { name: 'Beispielbriefe' })).toBeVisible();
  await expect(page.getByText(DISCLAIMER).first()).toBeVisible(); // app shell footer
  await page.getByRole('button', { name: /Ausländerbehörde/ }).click();
  await page.getByRole('button', { name: 'العربية' }).click();
  await expect(page.getByTestId('summary')).toHaveAttribute('dir', 'rtl');
  await expect(page.getByTestId('summary')).toHaveAttribute('lang', 'ar');
  await expect(page.getByTestId('actions')).toBeVisible();
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'In den Kalender (.ics)' }).click();
  expect((await download).suggestedFilename()).toBe('briefklar-frist.ics');
  await page.goto('/app/neu');
  await expect(page.getByRole('heading', { name: 'Beispielbriefe' })).toBeVisible(); // demo cannot upload
  // a logged-in user visiting the login page is sent on
  await page.goto('/app/login');
  await expect(page).not.toHaveURL(/\/login/);
});

/** Creates a fresh user through the admin API and logs in through the UI. */
async function loginAsNewUser(page: Page, request: APIRequestContext, tag: string): Promise<void> {
  const login = await request.post(`${BASE}/api/auth/login`, { ...ON_APP, data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD } });
  const token = (await login.json()).token as string;
  const user = { email: `${tag}.${run}@e2e.test`, name: 'Nutzer E2E', password: 'e2e-test-passwort-1', role: 'user', dailyLimit: 5 };
  expect((await request.post(`${BASE}/api/users`, { headers: { ...ON_APP.headers, Authorization: `Bearer ${token}` }, data: user })).status()).toBe(201);
  await page.goto('/app/login');
  await page.getByLabel('E-Mail').fill(user.email);
  await page.getByLabel('Passwort').fill(user.password);
  await page.getByRole('button', { name: 'Anmelden', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Neuer Brief' })).toBeVisible();
}

/** A one-page PDF with real text (pdftotext reads it, no OCR needed). ASCII only. */
function pdfWithText(lines: string[]): number[] {
  const content = `BT /F1 12 Tf 14 TL 50 780 Td ${lines.map((l) => `(${l}) '`).join(' ')} ET`;
  const objects = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>',
    `<< /Length ${content.length} >>\nstream\n${content}\nendstream`,
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
  ];
  let pdf = '%PDF-1.4\n';
  const offsets = objects.map((body, i) => {
    const at = pdf.length;
    pdf += `${i + 1} 0 obj\n${body}\nendobj\n`;
    return at;
  });
  const xref = pdf.length;
  pdf += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n`;
  pdf += offsets.map((o) => `${String(o).padStart(10, '0')} 00000 n \n`).join('');
  pdf += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return [...pdf].map((c) => c.charCodeAt(0));
}

test('the app is installable and never caches API responses', async ({ page, request }) => {
  const manifest = await (await request.get(`${BASE}/app/manifest.webmanifest`, ON_APP)).json();
  expect(manifest).toMatchObject({ short_name: 'Briefklar', start_url: '/app/', scope: '/app/', display: 'standalone' });
  expect(manifest.share_target.action).toBe('/app/share-target');
  for (const icon of manifest.icons as { src: string }[]) {
    const path = new URL(icon.src, `${APP}/app/manifest.webmanifest`).pathname;
    expect((await request.get(`${BASE}${path}`, ON_APP)).ok()).toBeTruthy();
  }
  const sw = await request.get(`${BASE}/app/sw.js`, ON_APP);
  expect(sw.headers()['cache-control']).toBe('no-cache');
  expect(await sw.text()).not.toContain('__BUILD__'); // stamped by the Docker build: every deploy is an update

  await page.goto('/app/login');
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute('href', 'manifest.webmanifest');
  const scope = await page.evaluate(async () => (await navigator.serviceWorker.ready).scope);
  expect(new URL(scope).pathname).toBe('/app/');
  await page.getByRole('button', { name: 'Als Demo anmelden' }).click();
  await expect(page.getByRole('heading', { name: 'Beispielbriefe' })).toBeVisible();
  const cached = await page.evaluate(async () => {
    const urls: string[] = [];
    for (const name of await caches.keys()) for (const req of await (await caches.open(name)).keys()) urls.push(new URL(req.url).pathname);
    return urls;
  });
  expect(cached).toContain('/app/');
  expect(cached.filter((u) => !u.startsWith('/app/'))).toEqual([]); // no /api, no landing page
});

test('offline the installed app still opens and says it needs a connection', async ({ page, context }) => {
  await page.goto('/app/login');
  await page.evaluate(async () => navigator.serviceWorker.ready);
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null);
  await context.setOffline(true);
  await page.reload();
  await expect(page.getByRole('button', { name: 'Als Demo anmelden' })).toBeVisible();
  await expect(page.getByText('Keine Verbindung')).toBeVisible();
  await context.setOffline(false);
});

test('a PDF shared from another app lands in the preview, redacted', async ({ page, request }) => {
  await loginAsNewUser(page, request, 'teilen');
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null);
  const pdf = pdfWithText(['Herrn', 'Karim Benali', 'Lindenweg 7', '51645 Gummersbach', 'Sehr geehrter Herr Benali,',
    'bitte zahlen Sie 93 Euro bis zum 15.11.2026.']);
  // what Android does after "Share -> Briefklar": a multipart POST navigation to the share target
  await page.evaluate((bytes) => {
    const form = Object.assign(document.createElement('form'), { action: '/app/share-target', method: 'post', enctype: 'multipart/form-data' });
    const input = Object.assign(document.createElement('input'), { type: 'file', name: 'file' });
    const files = new DataTransfer();
    files.items.add(new File([new Uint8Array(bytes)], 'bescheid.pdf', { type: 'application/pdf' }));
    input.files = files.files;
    form.append(input);
    document.body.append(form);
    form.submit();
  }, pdf);
  await expect(page).toHaveURL(/\/app\/neu$/);
  await expect(page.getByText('Vorschau: das bekommt die KI')).toBeVisible();
  const preview = page.getByTestId('preview-text');
  await expect(preview).toContainText('15.11.2026');
  await expect(preview).not.toContainText('Benali');
  // taken once and deleted: nothing from the letter stays on the device
  expect(await page.evaluate(async () => (await (await caches.open('briefklar-share')).keys()).length)).toBe(0);
});

test('a user pastes a letter, sees the redaction and gets an explanation with the name restored', async ({ page, request }) => {
  await loginAsNewUser(page, request, 'nutzer');
  await expect(page.getByText(DISCLAIMER).first()).toBeVisible();
  await expect(page.getByText('Foto aufnehmen')).toBeVisible();
  await expect(page.getByText('Datei wählen')).toBeVisible();

  await page.getByLabel('… oder Text einfügen').fill(
    'Herrn\nKarim Benali\nLindenweg 7\n51645 Gummersbach\nSehr geehrter Herr Benali,\nbitte überweisen Sie 93 Euro auf DE89 3704 0044 0532 0130 00 bis zum 15.11.2026.');
  await page.getByTestId('preview').click();
  await expect(page.getByText('Vorschau: das bekommt die KI')).toBeVisible();
  await expect(page.getByText('DE89 3704 0044 0532 0130 00')).toHaveCount(0);
  const preview = page.getByTestId('preview-text');
  await expect(preview).toContainText(/\[[A-Z_]+_\d+\]/);
  await expect(preview).not.toContainText('Benali');
  await page.getByTestId('explain').click();
  await expect(page.getByText('Musterbehörde (Testmodus)')).toBeVisible();
  await expect(page.getByLabel('Antwort-Entwurf')).toHaveValue(/Karim Benali/);
});
