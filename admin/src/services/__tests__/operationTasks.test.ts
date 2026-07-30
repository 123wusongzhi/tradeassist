import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  approveTask,
  cancelTask,
  createDraft,
  editDraft,
  executeTask,
  listAttempts,
  listEvents,
  listTasks,
  retryTask,
} from '../operationTasks';

const requestMock = vi.mocked(request);

describe('operation task API service', () => {
  it('queries tasks with keyset cursor params', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { items: [], hasMore: false, limit: 20 } });

    await listTasks({ status: 'pending_review', platform: 'douyin', taskType: 'product_content', cursor: 'next', limit: 20 });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks', {
      method: 'GET',
      params: { status: 'pending_review', platform: 'douyin', taskType: 'product_content', cursor: 'next', limit: 20 },
    });
  });

  it('sends idempotency key for draft writes', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'task-1' } });

    await createDraft('task-1', { payload: { title: '测试' }, changeReason: '修正内容', expectedTaskRevision: 3 }, 'key-123456');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/drafts', {
      method: 'POST',
      data: { payload: { title: '测试' }, changeReason: '修正内容', expectedTaskRevision: 3 },
      headers: { 'Idempotency-Key': 'key-123456' },
    });
  });

  it('sends expected draft version for latest draft edits', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'task-1' } });

    await editDraft('task-1', { payload: { title: '新内容' }, changeReason: '更新', expectedTaskRevision: 4, expectedDraftVersion: 2 }, 'key-edit');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/drafts/latest', {
      method: 'PATCH',
      data: { payload: { title: '新内容' }, changeReason: '更新', expectedTaskRevision: 4, expectedDraftVersion: 2 },
      headers: { 'Idempotency-Key': 'key-edit' },
    });
  });

  it('binds approval to draft version hash and revision', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'task-1' } });

    await approveTask('task-1', { draftVersion: 5, draftPayloadHash: 'hash', reason: '通过', expectedTaskRevision: 7 }, 'key-approve');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/approve', {
      method: 'POST',
      data: { draftVersion: 5, draftPayloadHash: 'hash', reason: '通过', expectedTaskRevision: 7 },
      headers: { 'Idempotency-Key': 'key-approve' },
    });
  });

  it('uses safe execution retry and cancel endpoints', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { ok: true } });

    await executeTask('task-1', { expectedTaskRevision: 1, adapterMode: 'local_draft_only' }, 'key-execute');
    await retryTask('task-1', { failedAttemptId: 'attempt-1', reason: '人工确认', expectedTaskRevision: 2 }, 'key-retry');
    await cancelTask('task-1', { reason: '不再需要', expectedTaskRevision: 3 }, 'key-cancel');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/execute', expect.objectContaining({ headers: { 'Idempotency-Key': 'key-execute' } }));
    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/retry', expect.objectContaining({ headers: { 'Idempotency-Key': 'key-retry' } }));
    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/cancel', expect.objectContaining({ headers: { 'Idempotency-Key': 'key-cancel' } }));
  });

  it('uses cursor contracts for attempts and events', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { items: [], hasMore: false, limit: 20 } });

    await listAttempts('task-1', { limit: 20, cursor: '2' });
    await listEvents('task-1', { limit: 30, afterSequence: 10 });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/attempts', { method: 'GET', params: { limit: 20, cursor: '2' } });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/operation-tasks/task-1/events', { method: 'GET', params: { limit: 30, afterSequence: 10 } });
  });
});
