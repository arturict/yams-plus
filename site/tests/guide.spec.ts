import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

const routes = ['/', '/privacy/', '/requirements/', '/install/', '/wizard/', '/prowlarr/', '/operations/', '/modules/', '/troubleshooting/', '/recovery/'];

test.beforeEach(async ({ page }) => {
  await page.route('https://umami.arturf.ch/script.js', route => route.fulfill({
    contentType: 'application/javascript',
    body: '',
  }));
});

test('guide renders, navigates, and has no serious accessibility failures', async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on('console', message => {
    if (message.type() === 'error') consoleErrors.push(message.text());
  });

  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'One wizard. One manual stop.' })).toBeVisible();
  await expect(page.locator('body')).toContainText('YAMS Plus');
  await expect(page.locator('[data-nextjs-dialog], .vite-error-overlay, #webpack-dev-server-client-overlay')).toHaveCount(0);
  expect((await page.locator('body').innerText()).trim().length).toBeGreaterThan(500);

  const accessibility = await new AxeBuilder({ page }).analyze();
  expect(accessibility.violations.filter(item => item.impact === 'critical' || item.impact === 'serious')).toEqual([]);
  expect(consoleErrors).toEqual([]);

  await page.screenshot({ path: '../acceptance/screenshots/guide-home.png', fullPage: true });
  await page.getByRole('link', { name: 'Install the beta' }).click();
  await expect(page).toHaveURL(/\/install\/$/);
  await expect(page.getByRole('heading', { name: 'Install' })).toBeVisible();
});

test('analytics are production-scoped and strip unsafe URL data', async ({ page }) => {
  await page.goto('/?utm_source=reddit&preview_token=do-not-collect#intro');

  const tracker = page.locator('script[data-website-id="f96b52f3-0218-4a31-ba34-0c3753efa7d8"]');
  await expect(tracker).toHaveAttribute('src', 'https://umami.arturf.ch/script.js');
  await expect(tracker).toHaveAttribute('data-domains', 'yamsplus-guide.vercel.app');
  await expect(tracker).toHaveAttribute('data-do-not-track', 'true');
  await expect(tracker).toHaveAttribute('data-exclude-hash', 'true');

  const sanitizedUrl = await page.evaluate(() => {
    const sanitize = (window as typeof window & {
      yamsPlusAnalyticsBeforeSend: (type: string, payload: { url: string }) => { url: string };
    }).yamsPlusAnalyticsBeforeSend;
    return sanitize('event', { url: '/install/?utm_source=reddit&utm_term=person%40example.com&preview_token=do-not-collect#secret' }).url;
  });
  expect(sanitizedUrl).toBe('/install/?utm_source=reddit');

  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'globalPrivacyControl', { configurable: true, value: true });
  });
  await page.reload();
  const blocked = await page.evaluate(() => (window as typeof window & {
    yamsPlusAnalyticsBeforeSend: (type: string, payload: { url: string }) => false | { url: string };
  }).yamsPlusAnalyticsBeforeSend('event', { url: '/' }));
  expect(blocked).toBe(false);
});

test('Reddit campaign attribution follows internal CTAs but never outbound links', async ({ page }) => {
  await page.addInitScript(() => {
    const analyticsWindow = window as typeof window & {
      capturedEvents: Array<{ name: string; data: Record<string, string> }>;
      umami: { track: (name: string, data: Record<string, string>) => void };
    };
    analyticsWindow.capturedEvents = [];
    analyticsWindow.umami = {
      track: (name, data) => analyticsWindow.capturedEvents.push({ name, data }),
    };
  });

  await page.goto('/?utm_source=reddit&utm_medium=social&utm_campaign=oss_launch&utm_content=selfhosted_post&preview_token=drop-me');

  const installLink = page.getByRole('link', { name: 'Install the beta' });
  const installUrl = new URL(await installLink.getAttribute('href') ?? '', 'http://guide.test');
  expect(installUrl.searchParams.get('utm_source')).toBe('reddit');
  expect(installUrl.searchParams.get('utm_campaign')).toBe('oss_launch');
  expect(installUrl.searchParams.get('utm_content')).toBe('selfhosted_post');
  expect(installUrl.searchParams.has('preview_token')).toBe(false);

  const outboundUrl = new URL(await page.getByRole('link', { name: 'YAMS', exact: true }).getAttribute('href') ?? '');
  expect(outboundUrl.search).toBe('');

  await installLink.evaluate(link => link.addEventListener('click', event => event.preventDefault(), { once: true }));
  await installLink.click();

  const events = await page.evaluate(() => (window as typeof window & {
    capturedEvents: Array<{ name: string; data: Record<string, string> }>;
  }).capturedEvents);
  expect(events).toContainEqual({
    name: 'landing-cta',
    data: expect.objectContaining({
      action: 'install',
      location: 'hero',
      target: 'install-guide',
    }),
  });
  expect(events.find(event => event.name === 'landing-cta')?.data).not.toHaveProperty('utm_source');
  expect(events.find(event => event.name === 'landing-cta')?.data).not.toHaveProperty('page');
});

test('landing milestones fire once and deeper guide pages emit no landing events', async ({ page }) => {
  await page.addInitScript(() => {
    const analyticsWindow = window as typeof window & {
      capturedEvents: Array<{ name: string; data: Record<string, string | number> }>;
      umami: { track: (name: string, data: Record<string, string | number>) => void };
    };
    analyticsWindow.capturedEvents = [];
    analyticsWindow.umami = {
      track: (name, data) => analyticsWindow.capturedEvents.push({ name, data }),
    };
  });

  await page.goto('/');
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect.poll(() => page.evaluate(() => (window as typeof window & {
    capturedEvents: Array<{ name: string }>;
  }).capturedEvents.filter(event => event.name === 'landing-scroll-depth').length)).toBe(4);
  await page.evaluate(() => {
    window.scrollTo(0, 0);
    window.scrollTo(0, document.documentElement.scrollHeight);
  });

  const landingEvents = await page.evaluate(() => (window as typeof window & {
    capturedEvents: Array<{ name: string; data: Record<string, string | number> }>;
  }).capturedEvents);
  expect(landingEvents.filter(event => event.name === 'landing-scroll-depth').map(event => event.data.depth)).toEqual([25, 50, 75, 100]);
  expect(new Set(landingEvents.filter(event => event.name === 'landing-section-view').map(event => event.data.section)).size)
    .toBe(landingEvents.filter(event => event.name === 'landing-section-view').length);

  await page.goto('/install/');
  const deeperPageLink = page.getByRole('link', { name: 'the only manual step' });
  await deeperPageLink.evaluate(link => link.addEventListener('click', event => event.preventDefault(), { once: true }));
  await deeperPageLink.click();
  const deeperPageEvents = await page.evaluate(() => (window as typeof window & {
    capturedEvents: Array<{ name: string }>;
  }).capturedEvents);
  expect(deeperPageEvents.filter(event => event.name.startsWith('landing-'))).toEqual([]);
});

for (const route of routes) {
  test(`${route} has meaningful content and no broken local links`, async ({ page, request }) => {
    await page.goto(route);
    await expect(page.locator('main')).toBeVisible();
    const links = await page.locator('main a[href^="/"]').evaluateAll(elements =>
      [...new Set(elements.map(element => (element as HTMLAnchorElement).pathname))],
    );
    for (const link of links) {
      const response = await request.get(link);
      expect(response.status(), `${route} -> ${link}`).toBeLessThan(400);
    }
  });
}
