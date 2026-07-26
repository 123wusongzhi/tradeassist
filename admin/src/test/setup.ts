import '@testing-library/jest-dom/vitest';
import React from 'react';
import { beforeEach, vi } from 'vitest';

const request = vi.fn();
const history = {
  location: { pathname: '/', search: '' },
  push: vi.fn(),
  replace: vi.fn(),
};

vi.mock('@umijs/max', () => ({
  request,
  history,
  Link: ({ children, to, ...props }: { children?: React.ReactNode; to?: string }) =>
    React.createElement('a', { href: to, ...props }, children),
  useModel: vi.fn(() => ({})),
  useAccess: vi.fn(() => ({})),
}));

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

Object.defineProperty(window, 'ResizeObserver', { writable: true, value: ResizeObserverMock });
Object.defineProperty(window, 'IntersectionObserver', { writable: true, value: IntersectionObserverMock });

beforeEach(() => {
  request.mockReset();
  history.push.mockReset();
  history.replace.mockReset();
  history.location.pathname = '/';
  history.location.search = '';
});
