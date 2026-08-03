#!/usr/bin/env node
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import {
  parseSafeTestDatabaseUrl,
  p9PostgresGoTestArgs,
  repoRoot,
} from './p9-postgres-contract.mjs';

export function runP9PostgresCI({
  env = process.env,
  spawn = spawnSync,
  logger = console,
} = {}) {
  const database = parseSafeTestDatabaseUrl(env.TEST_DATABASE_URL, env);
  if (!database.valid) {
    logger.error(`P9 PostgreSQL CI preflight failed: ${database.reason}`);
    return 1;
  }

  const commands = [
    { name: 'P9 PostgreSQL integration tests', args: p9PostgresGoTestArgs() },
    { name: 'P9 PostgreSQL race tests', args: p9PostgresGoTestArgs({ race: true }) },
  ];
  for (const command of commands) {
    logger.log(`Running ${command.name}...`);
    let result;
    try {
      result = spawn('go', command.args, {
        cwd: path.join(repoRoot, 'backend'),
        env,
        stdio: 'inherit',
      });
    } catch (error) {
      logger.error(`${command.name} could not start: ${error instanceof Error ? error.message : String(error)}`);
      return 1;
    }
    if (result.error) {
      logger.error(`${command.name} could not start: ${result.error.message}`);
      return 1;
    }
    if (result.signal) {
      logger.error(`${command.name} stopped by signal ${result.signal}`);
      return 1;
    }
    if (result.status !== 0) return Number.isInteger(result.status) ? result.status : 1;
  }

  logger.log('P9 PostgreSQL CI tests passed.');
  return 0;
}

const executedPath = process.argv[1] ? path.resolve(process.argv[1]) : '';
if (executedPath === fileURLToPath(import.meta.url)) {
  process.exitCode = runP9PostgresCI();
}
