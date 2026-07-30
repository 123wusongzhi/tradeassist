/// <reference types="node" />

import { defineConfig, devices } from '@playwright/test';

const isCI = !!process.env.CI;
const adminE2EPort = process.env.ADMIN_E2E_PORT || '8001';
const adminE2EBaseURL = `http://127.0.0.1:${adminE2EPort}`;

export default defineConfig({
  testDir: './admin/e2e/specs',
  outputDir: './test-results/admin-e2e',
  fullyParallel: false,
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  workers: isCI ? 1 : undefined,
  timeout: 45_000,
  expect: { timeout: 8_000 },
  reporter: isCI
    ? [
        ['list'],
        ['html', { outputFolder: 'playwright-report/admin-e2e', open: 'never' }],
        ['junit', { outputFile: 'test-results/admin-e2e-junit.xml' }],
      ]
    : [['list'], ['html', { outputFolder: 'playwright-report/admin-e2e', open: 'never' }]],
  use: {
    baseURL: adminE2EBaseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: isCI
      ? `pnpm build:admin && pnpm --filter @trademind/admin exec max preview --host 127.0.0.1 --port ${adminE2EPort}`
      : `pnpm dev:admin -- --host 127.0.0.1 --port ${adminE2EPort}`,
    url: adminE2EBaseURL,
    reuseExistingServer: !isCI,
    timeout: 120_000,
    env: {
      PORT: adminE2EPort,
      HOST: '127.0.0.1',
    },
  },
});
