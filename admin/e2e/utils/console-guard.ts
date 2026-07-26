import { expect, type ConsoleMessage, type Page } from '@playwright/test';

type GuardEntry = { type: string; text: string };

const allowedWarnings: RegExp[] = [
  /ResizeObserver loop completed with undelivered notifications/i,
  /Warning: Instance created by `useForm` is not connected to any Form element\. Forget to pass `form` prop\?/,
  /Warning: \[antd: Modal\] Static function can not consume context like dynamic theme\. Please use 'App' component instead\./,
];

export class ConsoleGuard {
  private readonly errors: GuardEntry[] = [];
  private readonly warnings: GuardEntry[] = [];

  constructor(private readonly page: Page) {}

  install() {
    this.page.on('pageerror', (error) => {
      this.errors.push({ type: 'pageerror', text: error.message });
    });
    this.page.on('console', (message) => this.recordConsole(message));
  }

  async expectNoFatalErrors() {
    const fatalErrors = this.errors.filter((entry) => !allowedWarnings.some((pattern) => pattern.test(entry.text)));
    const fatalWarnings = this.warnings.filter((entry) => !allowedWarnings.some((pattern) => pattern.test(entry.text)));
    expect([...fatalErrors, ...fatalWarnings], 'fatal console/page errors').toEqual([]);
  }

  entries() {
    return { errors: [...this.errors], warnings: [...this.warnings] };
  }

  private recordConsole(message: ConsoleMessage) {
    const text = message.text();
    if (message.type() === 'error') {
      this.errors.push({ type: 'console.error', text });
      return;
    }
    if (message.type() === 'warning' && /React|Warning|antd|Ant Design|useForm|unique "key"/i.test(text)) {
      this.warnings.push({ type: 'console.warning', text });
    }
  }
}
