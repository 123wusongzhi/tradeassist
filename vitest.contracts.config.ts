import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    globals: true,
    include: ['tests/contracts/**/*.test.ts'],
    clearMocks: true,
    restoreMocks: true,
  },
});
