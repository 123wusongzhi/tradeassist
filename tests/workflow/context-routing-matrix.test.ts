import { describe, expect, it } from 'vitest';
import { validateAgentContext } from '../../scripts/workflow/check-agent-context.mjs';
import { select } from '../../scripts/workflow/select-context.mjs';
import matrix from './context-routing-matrix.json';
import map from '../../config/agent/context-map.json';

describe('context routing matrix v2', () => {
  it('passes agent context validation', () => {
    const result = validateAgentContext();
    expect(result.failures, result.failures.join('\n')).toEqual([]);
  });

  it('documentation-only requires only docs-maintenance', () => {
    const result = select(map as any, {
      intent: 'documentation-only',
      files: ['docs/README.md'],
      riskFlags: [],
    });
    expect(result.error).toBeUndefined();
    expect(result.requiredContexts).toEqual(['docs-maintenance']);
  });

  it('small-admin-ui requires at most one context', () => {
    const result = select(map as any, {
      files: ['admin/src/pages/Products/DraftDetail/index.tsx'],
      riskFlags: [],
    });
    expect(result.error).toBeUndefined();
    expect(result.requiredContexts.length).toBeLessThanOrEqual(1);
    expect(result.requiredContexts[0]).toBe('admin-ui');
  });

  it('matrix scenarios are well-formed', () => {
    expect(matrix.version).toBe(2);
    expect(matrix.scenarios.length).toBeGreaterThanOrEqual(10);
    for (const scenario of matrix.scenarios) {
      expect(scenario.id).toBeTruthy();
      expect(['light', 'deep']).toContain(scenario.depth);
      expect(Array.isArray(scenario.expectedRequiredContexts)).toBe(true);
    }
  });
});
