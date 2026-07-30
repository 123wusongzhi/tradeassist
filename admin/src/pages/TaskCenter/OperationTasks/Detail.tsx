import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import { Button, Card, Descriptions, Form, Input, Modal, Select, Space, Spin, Table, Tabs, Tag, Timeline, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ErrorAlert, SectionCard, TaskJsonBlock, TmPageContainer } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  approveTask,
  cancelTask,
  createDraft,
  createOperationIdempotencyKey,
  editDraft,
  executeTask,
  extractOperationTaskAPIError,
  getTask,
  listAttempts,
  listDrafts,
  listEvents,
  rejectTask,
  retryTask,
  type ExecutionAttemptSummary,
  type OperationTaskAPIError,
  type OperationTaskDetail,
  type OperationTaskEventDTO,
  type PlatformDraftSummary,
} from '@/services/operationTasks';
import { formatDateTime } from '@/utils/formatTime';
import {
  NonProductionBoundary,
  OperationAttemptStatusTag,
  OperationDraftStatusTag,
  OperationTaskStatusTag,
  adapterModeLabel,
  copyableText,
  diffJSON,
  eventTypeLabel,
  jsonPreview,
  operationErrorMessage,
  parseJSONInput,
  platformLabel,
  renderOperationError,
  resultTypeLabel,
  safeMetadata,
  operationSourceLabel,
  taskTypeLabel,
} from './components/OperationTaskShared';

const { Text, Paragraph } = Typography;

type ModalKind = 'draft' | 'approve' | 'reject' | 'execute' | 'retry' | 'cancel';

function latestDraft(drafts: PlatformDraftSummary[]) {
  return [...drafts].sort((a, b) => b.draftVersion - a.draftVersion)[0];
}

function requiredReasonRule(label: string) {
  return [{ required: true, whitespace: true, message: `请填写${label}` }];
}

export default function OperationTaskDetailPage() {
  const params = useParams<{ taskId: string }>();
  const taskId = params.taskId || '';
  const [detail, setDetail] = useState<OperationTaskDetail | null>(null);
  const [drafts, setDrafts] = useState<PlatformDraftSummary[]>([]);
  const [attempts, setAttempts] = useState<ExecutionAttemptSummary[]>([]);
  const [events, setEvents] = useState<OperationTaskEventDTO[]>([]);
  const [attemptCursor, setAttemptCursor] = useState<string | undefined>();
  const [eventsSequence, setEventsSequence] = useState<number | undefined>();
  const [attemptHasMore, setAttemptHasMore] = useState(false);
  const [eventsHasMore, setEventsHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<OperationTaskAPIError | null>(null);
  const [modal, setModal] = useState<ModalKind | null>(null);
  const [jsonText, setJsonText] = useState('');
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [form] = Form.useForm();
  const requestSeq = useRef(0);

  const currentDraft = detail?.latestDraft || latestDraft(drafts);
  const failedAttempts = attempts.filter((attempt) => attempt.status === 'failed');

  const loadAll = useCallback(async () => {
    if (!taskId) return;
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const [nextDetail, draftPage, attemptPage, eventPage] = await Promise.all([
        getTask(taskId),
        listDrafts(taskId, 50),
        listAttempts(taskId, { limit: 20 }),
        listEvents(taskId, { limit: 30 }),
      ]);
      if (requestSeq.current !== seq) return;
      setDetail(nextDetail);
      setDrafts([...draftPage.items].sort((a, b) => b.draftVersion - a.draftVersion));
      setAttempts(attemptPage.items);
      setEvents(eventPage.items);
      setAttemptCursor(attemptPage.nextCursor);
      setEventsSequence(eventPage.nextSequence);
      setAttemptHasMore(attemptPage.hasMore);
      setEventsHasMore(eventPage.hasMore);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractOperationTaskAPIError(e));
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const openModal = (kind: ModalKind) => {
    setModal(kind);
    setJsonError(null);
    form.resetFields();
    if (kind === 'draft') {
      setJsonText(JSON.stringify(detail?.payload ?? {}, null, 2));
      form.setFieldsValue({ changeReason: '' });
    }
    if (kind === 'execute') form.setFieldsValue({ adapterMode: 'local_draft_only' });
    if (kind === 'retry') form.setFieldsValue({ failedAttemptId: failedAttempts[0]?.attemptId, reason: '' });
  };

  const closeModal = () => {
    if (actionLoading) return;
    setModal(null);
    setJsonError(null);
  };

  const refreshAfterAction = async (successText: string) => {
    message.success(successText);
    closeModal();
    await loadAll();
  };

  const runAction = async () => {
    if (!detail || !modal) return;
    setActionLoading(true);
    setError(null);
    try {
      if (modal === 'draft') {
        const parsed = parseJSONInput(jsonText);
        if (!parsed.ok) {
          setJsonError(parsed.message);
          setActionLoading(false);
          return;
        }
        const values = await form.validateFields();
        const key = createOperationIdempotencyKey(currentDraft ? 'edit-draft' : 'create-draft');
        if (currentDraft) {
          await editDraft(taskId, {
            payload: parsed.value,
            changeReason: values.changeReason,
            expectedTaskRevision: detail.revision,
            expectedDraftVersion: currentDraft.draftVersion,
          }, key);
        } else {
          await createDraft(taskId, {
            payload: parsed.value,
            changeReason: values.changeReason,
            expectedTaskRevision: detail.revision,
          }, key);
        }
        await refreshAfterAction('草稿已保存，最新状态已刷新。');
      }
      if (modal === 'approve') {
        const values = await form.validateFields();
        if (!currentDraft) throw new Error('当前没有可审核草稿');
        await approveTask(taskId, {
          draftVersion: currentDraft.draftVersion,
          draftPayloadHash: currentDraft.payloadHash,
          reason: values.reason,
          comment: values.comment,
          expectedTaskRevision: detail.revision,
        }, createOperationIdempotencyKey('approve'));
        await refreshAfterAction('已批准最新草稿。');
      }
      if (modal === 'reject') {
        const values = await form.validateFields();
        if (!currentDraft) throw new Error('当前没有可拒绝草稿');
        await rejectTask(taskId, {
          draftVersion: currentDraft.draftVersion,
          draftPayloadHash: currentDraft.payloadHash,
          reason: values.reason,
          comment: values.comment,
          expectedTaskRevision: detail.revision,
        }, createOperationIdempotencyKey('reject'));
        await refreshAfterAction('已拒绝最新草稿。');
      }
      if (modal === 'execute') {
        const values = await form.validateFields();
        await executeTask(taskId, {
          expectedTaskRevision: detail.revision,
          adapterMode: values.adapterMode,
        }, createOperationIdempotencyKey('execute'));
        await refreshAfterAction('草稿生成请求已提交。');
      }
      if (modal === 'retry') {
        const values = await form.validateFields();
        await retryTask(taskId, {
          failedAttemptId: values.failedAttemptId,
          reason: values.reason,
          expectedTaskRevision: detail.revision,
        }, createOperationIdempotencyKey('retry'));
        await refreshAfterAction('人工重试请求已提交。');
      }
      if (modal === 'cancel') {
        const values = await form.validateFields();
        await cancelTask(taskId, {
          reason: values.reason,
          expectedTaskRevision: detail.revision,
        }, createOperationIdempotencyKey('cancel'));
        await refreshAfterAction('任务已取消，最新状态已刷新。');
      }
    } catch (e) {
      const next = extractOperationTaskAPIError(e);
      setError(next);
      message.error(operationErrorMessage(next));
      if (next.errorCode?.includes('conflict') || next.errorCode?.includes('mismatch') || next.errorCode === 'state_conflict') {
        await loadAll();
      }
    } finally {
      setActionLoading(false);
    }
  };

  const loadMoreAttempts = async () => {
    if (!attemptCursor) return;
    const page = await listAttempts(taskId, { limit: 20, cursor: attemptCursor });
    setAttempts((prev) => [...prev, ...page.items]);
    setAttemptCursor(page.nextCursor);
    setAttemptHasMore(page.hasMore);
  };

  const loadMoreEvents = async () => {
    if (!eventsSequence) return;
    const page = await listEvents(taskId, { limit: 30, afterSequence: eventsSequence });
    setEvents((prev) => [...prev, ...page.items]);
    setEventsSequence(page.nextSequence);
    setEventsHasMore(page.hasMore);
  };

  const draftColumns: ColumnsType<PlatformDraftSummary> = [
    { title: '版本', dataIndex: 'draftVersion', width: 90, render: (v) => `v${v}` },
    { title: '状态', dataIndex: 'status', width: 120, render: (v) => <OperationDraftStatusTag status={String(v)} /> },
    { title: 'Payload Hash', dataIndex: 'payloadHash', render: (v) => copyableText(String(v), 18) },
    { title: '变更原因', dataIndex: 'changeReason', ellipsis: true, render: (v) => v || '—' },
    { title: '创建人', dataIndex: 'createdBy', render: (v) => copyableText(String(v || ''), 10) },
    { title: '创建时间', dataIndex: 'createdAt', render: (v) => formatDateTime(String(v)) },
  ];

  const attemptColumns: ColumnsType<ExecutionAttemptSummary> = [
    { title: 'Attempt', dataIndex: 'attemptNumber', width: 100 },
    { title: '状态', dataIndex: 'status', width: 120, render: (v) => <OperationAttemptStatusTag status={String(v)} /> },
    { title: 'Adapter Mode', dataIndex: 'adapterMode', render: (v) => adapterModeLabel(String(v)) },
    { title: '批准草稿', dataIndex: 'approvedDraftVersion', render: (_, row) => `v${row.approvedDraftVersion} / ${row.approvedDraftPayloadHash || '—'}` },
    { title: '执行草稿', dataIndex: 'executedDraftVersion', render: (_, row) => row.executedDraftVersion ? `v${row.executedDraftVersion} / ${row.executedDraftPayloadHash || '—'}` : '—' },
    { title: '结果', dataIndex: 'resultType', render: (v) => resultTypeLabel(String(v || '')) },
    { title: 'Request ID', dataIndex: 'requestId', render: (v) => copyableText(String(v || ''), 14) },
    { title: '开始时间', dataIndex: 'startedAt', render: (v) => formatDateTime(String(v || '')) },
    { title: '结束时间', dataIndex: 'finishedAt', render: (v) => formatDateTime(String(v || '')) },
  ];

  const modalTitle = useMemo(() => {
    if (modal === 'draft') return currentDraft ? '编辑最新草稿' : '创建草稿';
    if (modal === 'approve') return '确认批准';
    if (modal === 'reject') return '拒绝草稿';
    if (modal === 'execute') return '执行草稿生成';
    if (modal === 'retry') return '人工重试';
    if (modal === 'cancel') return '取消运营任务';
    return '';
  }, [currentDraft, modal]);

  if (loading && !detail) {
    return <Spin fullscreen tip="正在加载运营任务详情" />;
  }

  if (!detail && error) {
    return (
      <TmPageContainer title="运营任务详情">
        <ErrorAlert title={operationErrorMessage(error)} actionHint={error.traceId ? `排查编号：${error.traceId}` : '请返回列表后重试。'} />
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer
      title={PAGE_COPY.operationTasks.title}
      subTitle="运营任务详情"
      extra={
        <Space wrap>
          <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/ops/task-center/operation-tasks')}>返回列表</Button>
          <Button icon={<ReloadOutlined />} onClick={() => void loadAll()} loading={loading}>刷新</Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderOperationError(error)}
        <NonProductionBoundary />
        {detail ? (
          <SectionCard
            title={detail.title}
            description={detail.summary || '运营任务概要'}
            headerExtra={<OperationTaskStatusTag status={detail.status} />}
          >
            <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
              <Descriptions.Item label="任务 ID">{copyableText(detail.id, 18)}</Descriptions.Item>
              <Descriptions.Item label="任务类型">{taskTypeLabel(detail.taskType)}</Descriptions.Item>
              <Descriptions.Item label="平台">{platformLabel(detail.platform)}</Descriptions.Item>
              <Descriptions.Item label="优先级">{detail.priority}</Descriptions.Item>
              <Descriptions.Item label="Revision">{detail.revision}</Descriptions.Item>
              <Descriptions.Item label="来源">{operationSourceLabel(String((detail as Record<string, unknown>)['source' + 'Type'] || ''))} / {detail.sourceReference || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建人">{copyableText(detail.createdBy, 12)}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(detail.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="更新时间">{formatDateTime(detail.updatedAt)}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
        ) : null}

        {detail ? (
          <Card>
            <Space wrap>
              <Button disabled={!detail.allowedActions.canEditDraft} onClick={() => openModal('draft')}>编辑草稿</Button>
              <Button type="primary" disabled={!detail.allowedActions.canApprove || !currentDraft} onClick={() => openModal('approve')}>确认批准</Button>
              <Button danger disabled={!detail.allowedActions.canReject || !currentDraft} onClick={() => openModal('reject')}>拒绝</Button>
              <Button disabled={!detail.allowedActions.canExecute} onClick={() => openModal('execute')}>执行草稿生成</Button>
              <Button disabled={!detail.allowedActions.canRetry || failedAttempts.length === 0} onClick={() => openModal('retry')}>人工重试</Button>
              <Button danger disabled={!detail.allowedActions.canCancel} onClick={() => openModal('cancel')}>取消任务</Button>
            </Space>
            <Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
              按钮状态来自后端 allowedActions；提交时后端仍会重新校验权限、状态、Revision 与幂等边界。
            </Paragraph>
          </Card>
        ) : null}

        {detail ? (
          <Tabs
            items={[
              {
                key: 'drafts',
                label: '草稿版本',
                children: (
                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    <SectionCard title="最新草稿" description="仅表示本地、模拟或沙箱草稿，不代表已发布商品。">
                      {currentDraft ? (
                        <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
                          <Descriptions.Item label="版本">v{currentDraft.draftVersion}</Descriptions.Item>
                          <Descriptions.Item label="状态"><OperationDraftStatusTag status={currentDraft.status} /></Descriptions.Item>
                          <Descriptions.Item label="Payload Hash">{copyableText(currentDraft.payloadHash, 18)}</Descriptions.Item>
                          <Descriptions.Item label="变更原因">{currentDraft.changeReason || '—'}</Descriptions.Item>
                          <Descriptions.Item label="创建人">{copyableText(currentDraft.createdBy, 12)}</Descriptions.Item>
                          <Descriptions.Item label="更新时间">{formatDateTime(currentDraft.updatedAt)}</Descriptions.Item>
                        </Descriptions>
                      ) : <Text type="secondary">暂无草稿版本。</Text>}
                    </SectionCard>
                    <Table rowKey="draftId" columns={draftColumns} dataSource={drafts} pagination={false} scroll={{ x: 900 }} />
                    <SectionCard title="任务 Payload 预览" description="当前后端未返回历史草稿完整 Payload，此处展示任务载荷并在编辑时生成安全本地差异。">
                      {jsonPreview(detail.payload)}
                    </SectionCard>
                  </Space>
                ),
              },
              {
                key: 'attempts',
                label: '执行历史',
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Table rowKey="attemptId" columns={attemptColumns} dataSource={attempts} pagination={false} scroll={{ x: 1280 }} />
                    {attemptHasMore ? <Button onClick={() => void loadMoreAttempts()}>加载更多执行记录</Button> : null}
                  </Space>
                ),
              },
              {
                key: 'events',
                label: '审计时间线',
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Timeline
                      items={events.map((event) => ({
                        key: event.eventId,
                        children: (
                          <Space direction="vertical" size={4} style={{ width: '100%' }}>
                            <Space wrap>
                              <Text strong>#{event.sequence}</Text>
                              <Tag>{eventTypeLabel(event.eventType)}</Tag>
                              <Text type="secondary">{formatDateTime(event.occurredAt)}</Text>
                            </Space>
                            <Text>状态：{event.beforeState || '—'} → {event.afterState || '—'}</Text>
                            <Text>操作者：{event.actorType}{event.actorId ? ` / ${event.actorId}` : ''}</Text>
                            <Text>草稿版本：{event.draftVersion || '—'}；Request ID：{event.requestId || '—'}</Text>
                            {event.reason ? <Text>原因：{event.reason}</Text> : null}
                            <TaskJsonBlock title="安全 Metadata" value={safeMetadata(event.metadata)} maxHeight={160} last />
                          </Space>
                        ),
                      }))}
                    />
                    {eventsHasMore ? <Button onClick={() => void loadMoreEvents()}>加载更多事件</Button> : null}
                  </Space>
                ),
              },
            ]}
          />
        ) : null}
      </Space>

      <Modal title={modalTitle} open={!!modal} onCancel={closeModal} onOk={() => void runAction()} confirmLoading={actionLoading} okText={modal === 'approve' ? '确认批准' : '确认'} cancelText="取消" width={760} destroyOnClose>
        {modal === 'draft' ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Paragraph>保存将生成或更新最新草稿版本。若任务已批准，编辑后需重新审核。</Paragraph>
            <Input.TextArea value={jsonText} onChange={(e) => { setJsonText(e.target.value); setJsonError(null); }} rows={12} aria-label="草稿 JSON" />
            {jsonError ? <Text type="danger">JSON 格式错误：{jsonError}</Text> : null}
            <Form form={form} layout="vertical">
              <Form.Item name="changeReason" label="变更原因" rules={requiredReasonRule('变更原因')}>
                <Input.TextArea rows={3} maxLength={500} showCount />
              </Form.Item>
            </Form>
            <TaskJsonBlock title="安全差异预览（仅展示变化行，最多 200 行）" value={diffJSON(detail?.payload, parseJSONInput(jsonText).ok ? parseJSONInput(jsonText).value : {})} maxHeight={220} />
          </Space>
        ) : null}

        {modal === 'approve' || modal === 'reject' ? (
          <Form form={form} layout="vertical">
            <Paragraph>当前操作绑定最新草稿 v{currentDraft?.draftVersion || '—'} / {currentDraft?.payloadHash || '—'}，不会自动发布或上架商品。</Paragraph>
            <Form.Item name="reason" label={modal === 'approve' ? '批准说明' : '拒绝原因'} rules={requiredReasonRule(modal === 'approve' ? '批准说明' : '拒绝原因')}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
            <Form.Item name="comment" label="补充说明">
              <Input.TextArea rows={2} maxLength={500} showCount />
            </Form.Item>
          </Form>
        ) : null}

        {modal === 'execute' ? (
          <Form form={form} layout="vertical">
            <NonProductionBoundary />
            <Paragraph style={{ marginTop: 12 }}>执行仅生成本地、模拟或沙箱草稿，不调用真实平台发布或上架。</Paragraph>
            <Form.Item name="adapterMode" label="草稿生成模式" rules={[{ required: true, message: '请选择草稿生成模式' }]}>
              <Select
                options={[
                  { value: 'local_draft_only', label: adapterModeLabel('local_draft_only') },
                  { value: 'mock', label: adapterModeLabel('mock') },
                  { value: 'sandbox', label: adapterModeLabel('sandbox') },
                ]}
              />
            </Form.Item>
          </Form>
        ) : null}

        {modal === 'retry' ? (
          <Form form={form} layout="vertical">
            <Paragraph>仅对明确失败的 Attempt 发起一次人工重试，不会自动连续重试。</Paragraph>
            <Form.Item name="failedAttemptId" label="失败 Attempt" rules={[{ required: true, message: '请选择失败 Attempt' }]}>
              <Select options={failedAttempts.map((attempt) => ({ value: attempt.attemptId, label: `#${attempt.attemptNumber} / ${attempt.status}` }))} />
            </Form.Item>
            <Form.Item name="reason" label="重试原因" rules={requiredReasonRule('重试原因')}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          </Form>
        ) : null}

        {modal === 'cancel' ? (
          <Form form={form} layout="vertical">
            <Paragraph>取消后该任务不能继续执行；已完成或已取消的任务由后端拒绝重复取消。</Paragraph>
            <Form.Item name="reason" label="取消原因" rules={requiredReasonRule('取消原因')}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          </Form>
        ) : null}
      </Modal>
    </TmPageContainer>
  );
}
