#!/usr/bin/env node
import { spawnSync } from 'node:child_process';

const args = [
  'test',
  './internal/pkg/httpclient',
  './internal/modules/webhook',
  './internal/modules/ordersync',
  './internal/modules/inventory',
  './internal/modules/aiproducttext',
  './internal/modules/aiproductimage',
  './internal/modules/files',
  './internal/modules/securitymod',
  './internal/modules/auth',
  './internal/modules/alerting',
];

const result = spawnSync('go', args, {
  cwd: new URL('../backend/', import.meta.url),
  stdio: 'inherit',
  shell: process.platform === 'win32',
});

if (result.status !== 0) process.exit(result.status || 1);
console.log(JSON.stringify({ phase: 'P5.2', smoke: 'passed', failed: 0 }));
