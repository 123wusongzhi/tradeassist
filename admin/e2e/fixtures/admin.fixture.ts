import { test as base } from '@playwright/test';
import { AdminPage } from '../pages/admin.page';

type AdminFixtures = {
  admin: AdminPage;
  autoAdminGuards: void;
};

export const test = base.extend<AdminFixtures>({
  admin: async ({ page }, use) => {
    await use(new AdminPage(page));
  },
  autoAdminGuards: [
    async ({ admin }, use) => {
      await admin.setup();
      await use();
      await admin.writeGuard.expectNoUnexpectedWrites();
      await admin.consoleGuard.expectNoFatalErrors();
    },
    { auto: true },
  ],
});

export { expect } from '@playwright/test';
