import { Alert, Button, Form, Input, Modal, Segmented, Space, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { TaobaoTmallLoginPanel } from '@/pages/Collect/components/TaobaoTmallLoginPanel';
import type { ProviderTaobaoTmallAuthStatus } from '@/services/collectAuth';
import { createCollectTask, fetchCollectEnginesStatus, type CollectEnginesStatus } from '@/services/collectTasks';
import { COLLECT_SUCCESS_SHOP_HINT } from '@/constants/copywriting';
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import { classifyTaobaoTmallUrl, taobaoTmallUrlHint, validateTaobaoTmallUrl } from '@/utils/taobaoTmallUrl';
import { collectEngineOptions, findCollectEngineStatus, type CollectEngine } from '@/utils/collectEngine';
import { fetchDefaultCollectEngine } from '@/services/collectEngine';

type Props = {
  open: boolean;
  onClose: () => void;
  onSubmitted?: () => void;
};

export function TaobaoTmallCollectModal({ open, onClose, onSubmitted }: Props) {
  const [form] = Form.useForm<{ url: string }>();
  const url = Form.useWatch('url', form);
  const [submitting, setSubmitting] = useState(false);
  const [authStatus, setAuthStatus] = useState<ProviderTaobaoTmallAuthStatus | null>(null);
  const [engine, setEngine] = useState<CollectEngine>('opencli');
  const [engineResolving, setEngineResolving] = useState(true);
  const [engineStatus, setEngineStatus] = useState<CollectEnginesStatus | null>(null);

  useEffect(() => {
    if (!open) {
      form.resetFields();
      setAuthStatus(null);
      setEngine('opencli');
      setEngineResolving(true);
      setEngineStatus(null);
      return;
    }
    let cancelled = false;
    setEngineResolving(true);
    void fetchCollectEnginesStatus()
      .then(async (status) => {
        if (!cancelled) setEngineStatus(status);
        return fetchDefaultCollectEngine(status);
      })
      .then((value) => {
        if (!cancelled) setEngine(value);
      })
      .catch(() => {
        if (!cancelled) setEngine('playwright');
      })
      .finally(() => {
        if (!cancelled) setEngineResolving(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, form]);

  const urlHint = useMemo(() => {
    const u = url?.trim();
    if (!u) return null;
    return taobaoTmallUrlHint(u);
  }, [url]);

  const urlType = url?.trim() ? classifyTaobaoTmallUrl(url.trim()) : null;
  const openCliStatus = findCollectEngineStatus(engineStatus, 'opencli');
  const openCliSelectable = Boolean(openCliStatus?.enabled && openCliStatus.configured);
  const canSubmit = urlType === 'product_detail' && (engine !== 'opencli' || openCliSelectable);

  const handleSubmit = async () => {
    const vals = await form.validateFields();
    const raw = vals.url?.trim();
    if (!raw || !validateTaobaoTmallUrl(raw)) {
      message.warning('请输入有效的淘宝/天猫商品详情页链接');
      return;
    }
    setSubmitting(true);
    try {
      await createCollectTask({
        source: 'taobao_tmall',
        url: raw,
        useBrowserProfile: engine === 'playwright',
        engine,
      });
      message.success(COLLECT_SUCCESS_SHOP_HINT, 6);
      onSubmitted?.();
      onClose();
    } catch (e) {
      message.error(mapCollectErrorMessage(e, 'taobao_tmall', engine));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="淘宝/天猫采集"
      open={open}
      onCancel={onClose}
      width={680}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" loading={submitting} disabled={!canSubmit} onClick={() => void handleSubmit()}>
            开始采集
          </Button>
        </Space>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        {engine === 'opencli'
          ? '默认使用宿主机 OpenCLI 和当前浏览器登录状态采集；如需使用采集服务内置浏览器，可切换到 Playwright 备用引擎。'
          : 'Playwright 使用采集服务内置浏览器。部分商品需要先完成登录或安全验证，再重新采集。'}
      </Typography.Paragraph>
      <Form form={form} layout="vertical">
        <Form.Item label="商品链接" name="url" rules={[{ required: true, message: '请填写淘宝/天猫商品链接' }]}>
          <Input placeholder="https://item.taobao.com/item.htm?id=..." />
        </Form.Item>
        <Form.Item label="采集引擎" style={{ marginBottom: 8 }}>
          <Segmented<CollectEngine>
            value={engine}
            disabled={engineResolving}
            onChange={(v) => {
              setEngine(v);
              setAuthStatus(null);
            }}
            options={collectEngineOptions(engineStatus)}
          />
          {engine === 'opencli' ? (
            <Alert
              style={{ marginTop: 8 }}
              showIcon
              type={openCliStatus?.ready ? 'success' : 'warning'}
              message={
                engineResolving
                  ? '正在检测 OpenCLI Bridge'
                  : openCliStatus?.ready
                    ? 'OpenCLI Bridge 已就绪'
                    : openCliStatus?.message || 'OpenCLI Bridge 当前不可用'
              }
              description={
                engineResolving
                  ? '检测完成后会按采集设置选择默认引擎。'
                  : openCliStatus?.ready
                    ? '使用宿主机已登录浏览器采集；Playwright Collector 会继续独立运行，随时可切换。'
                    : '请在宿主机启动 OpenCLI Bridge，或临时切换为 Playwright。系统不会自动切换引擎。'
              }
            />
          ) : null}
        </Form.Item>
      </Form>
      {urlHint ? (
        <Alert
          type={urlType === 'product_detail' ? 'success' : urlType === 'unsupported_taobao' ? 'error' : 'warning'}
          showIcon
          message={urlHint}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      {!engineResolving && engine === 'playwright' ? (
        <TaobaoTmallLoginPanel loginUrl={url?.trim()} onAuthChange={setAuthStatus} />
      ) : null}
      {!engineResolving && engine === 'playwright' && authStatus && !authStatus.loggedIn ? (
        <Typography.Text type="warning" style={{ display: 'block', marginTop: 12 }}>
          当前未检测到登录态，部分商品可能采集失败，建议先完成登录后再采集。
        </Typography.Text>
      ) : null}
    </Modal>
  );
}
