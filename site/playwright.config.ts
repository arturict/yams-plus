import { defineConfig, devices } from '@playwright/test';

const remoteBaseURL = process.env.PLAYWRIGHT_BASE_URL;
const localPort = process.env.PLAYWRIGHT_PORT ?? '4321';
const localBaseURL = `http://127.0.0.1:${localPort}`;

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: true,
  retries: 0,
  reporter: 'line',
  use: {
    baseURL: remoteBaseURL ?? localBaseURL,
    trace: 'retain-on-failure',
    ...devices['Desktop Chrome'],
  },
  webServer: remoteBaseURL ? undefined : {
    command: `npm run build && npm run preview -- --host 127.0.0.1 --port ${localPort}`,
    url: localBaseURL,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
