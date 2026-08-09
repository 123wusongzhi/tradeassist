/**
 * Legacy v1 matrix tests retired.
 * Active coverage: tests/workflow/context-routing-matrix.test.ts
 */
import { describe, expect, it } from 'vitest';
import { validateAgentContext } from '../../scripts/workflow/check-agent-context.mjs';

describe('workflow skill trigger matrix (legacy bridge)', () => {
  it('defers to agent context v2 checker', () => {
    const result = validateAgentContext();
    expect(result.failures, result.failures.join('\n')).toEqual([]);
  });
});
