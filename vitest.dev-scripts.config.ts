import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    include: ['scripts/**/*.test.ts', 'admin/config/**/*.test.ts'],
    threads: false,
  },
});
