import { describe, expect, it } from 'vitest';

import {
  resolveDevServicePortMap,
  resolveDevServicePorts,
  resolveDockerPublishPortMap,
} from '../port-cleanup.js';

describe('optional collector ports', () => {
  it('omits the Playwright collector from default local and Docker ports', () => {
    const overrides = {} as NodeJS.ProcessEnv;

    expect(resolveDevServicePortMap(undefined, overrides)).toEqual({
      backend: 8080,
      admin: 8000,
    });
    expect(resolveDevServicePorts(undefined, overrides)).toEqual([8080, 8000]);
    expect(resolveDockerPublishPortMap(undefined, overrides)).toEqual({
      backend: 8080,
      admin: 8000,
    });
  });

  it('includes configured collector ports after explicit enablement', () => {
    const overrides = {
      COLLECTOR_PLAYWRIGHT_ENABLED: 'true',
      COLLECTOR_HTTP_ADDR: '127.0.0.1:3301',
      COLLECTOR_PUBLISH_PORT: '4301',
    } as NodeJS.ProcessEnv;

    expect(resolveDevServicePortMap(undefined, overrides).collector).toBe(3301);
    expect(resolveDevServicePorts(undefined, overrides)).toContain(3301);
    expect(resolveDockerPublishPortMap(undefined, overrides).collector).toBe(4301);
  });
});
