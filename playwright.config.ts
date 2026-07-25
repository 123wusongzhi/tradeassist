/// <reference types="node" />

import { defineConfig, devices } from '@playwright/test';

const isCI = !!process.env.CI;

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
    baseURL: 'http://127.0.0.1:8001',
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
      ? 'pnpm --filter @trademind/admin exec max preview --host 127.0.0.1 --port 8001'
      : 'pnpm dev:admin -- --host 127.0.0.1 --port 8001',
    url: 'http://127.0.0.1:8001',
    reuseExistingServer: !isCI,
    timeout: 120_000,
    env: {
      PORT: '8001',
      HOST: '127.0.0.1',
    },
  },
});
