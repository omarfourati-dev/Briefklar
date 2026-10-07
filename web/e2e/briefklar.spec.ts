import { expect, test } from '@playwright/test';

const ADMIN_EMAIL = process.env['E2E_ADMIN_EMAIL'] ?? 'admin@briefklar.test';
const ADMIN_PASSWORD = process.env['E2E_ADMIN_PASSWORD'] ?? 'local-admin-password-123';
const run = Date.now().toString(36);
const DISCLAIMER = 'Keine Rechtsberatung. Im Zweifel bei der Behörde nachfragen.';

test('landing page explains Briefklar and leads to the demo', async ({ page, request }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Briefklar/);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Was will das Amt');
  await expect(page.getByText('Keine Rechtsberatung').first()).toBeVisible();
  JSON.parse((await page.locator('script[type="application/ld+json"]').first().textContent()) ?? '');
  await page.getByRole('link', { name: 'Demo ansehen' }).first().click();
  await expect(page.getByRole('button', { name: 'Als Demo anmelden' })).toBeVisible();
  await expect(page.getByText(DISCLAIMER)).toBeVisible();
  expect((await request.get('/llms.txt')).ok()).toBeTruthy();
  expect((await request.get('/healthz')).ok()).toBeTruthy();
  expect((await request.get('/metrics')).ok()).toBeTruthy(); // direct; Caddy blocks it in production
});

test('login page shows the demo credentials and the Angular app is styled', async ({ page }) => {
  await page.goto('/app/login');
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

test('a user pastes a letter, sees the redaction and gets an explanation with the name restored', async ({ page, request }) => {
  const login = await request.post('/api/auth/login', { data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD } });
  const token = (await login.json()).token as string;
  const user = { email: `nutzer.${run}@e2e.test`, name: 'Nutzer E2E', password: 'e2e-test-passwort-1', role: 'user', dailyLimit: 5 };
  expect((await request.post('/api/users', { headers: { Authorization: `Bearer ${token}` }, data: user })).status()).toBe(201);

  await page.goto('/app/login');
  await page.getByLabel('E-Mail').fill(user.email);
  await page.getByLabel('Passwort').fill(user.password);
  await page.getByRole('button', { name: 'Anmelden', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Neuer Brief' })).toBeVisible();
  await expect(page.getByText(DISCLAIMER)).toBeVisible();
  await expect(page.getByText('Foto aufnehmen')).toBeVisible();
  await expect(page.getByText('Datei wählen')).toBeVisible();

  await page.getByLabel('… oder Text einfügen').fill(
    'Herrn\nKarim Benali\nLindenweg 7\n51645 Gummersbach\nSehr geehrter Herr Benali,\nbitte überweisen Sie 93 Euro auf DE89 3704 0044 0532 0130 00 bis zum 15.11.2026.');
  await page.getByTestId('preview').click();
  await expect(page.getByText('Vorschau: das bekommt die KI')).toBeVisible();
  await expect(page.getByText('DE89 3704 0044 0532 0130 00')).toHaveCount(0);
  await page.getByTestId('explain').click();
  await expect(page.getByText('Musterbehörde (Testmodus)')).toBeVisible();
  await expect(page.getByLabel('Antwort-Entwurf')).toHaveValue(/Karim Benali/);
});
