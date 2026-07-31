import {
  CheckCircleOutlined,
  ChromeOutlined,
  CopyOutlined,
  DeleteOutlined,
  KeyOutlined,
  ReloadOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Col,
  Popconfirm,
  Row,
  Skeleton,
  Space,
  Steps,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { EmptyState, SectionCard, TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createBrowserExtensionPairing,
  queryBrowserExtensionDevices,
  revokeBrowserExtensionDevice,
  type BrowserExtensionDeviceRow,
  type BrowserExtensionPairing,
} from '@/services/collectBrowserExtension';
import { buildBrowserExtensionConnectionInfo, isRemoteHttpOrigin } from '@/utils/browserExtensionPairing';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';

const { Text, Title } = Typography;

async function copyTextToClipboard(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  textarea.remove();
}

function deviceStatusTag(status: string) {
  if (status === 'active') {
    return <Tag color="success">有效</Tag>;
  }
  if (status === 'expired') {
    return <Tag color="warning">已过期</Tag>;
  }
  return <Tag>已撤销</Tag>;
}

export default function CollectBrowserExtensionPage() {
  const { can } = usePermission();
  const canManageExtension = can(PERMISSIONS.PRODUCT_WRITE);
  const remoteHttp = isRemoteHttpOrigin(window.location.origin);

  const [devices, setDevices] = useState<BrowserExtensionDeviceRow[]>([]);
  const [devicesError, setDevicesError] = useState('');
  const [devicesLoading, setDevicesLoading] = useState(true);
  const [pairing, setPairing] = useState<BrowserExtensionPairing | null>(null);
  const [generating, setGenerating] = useState(false);
  const [revokingId, setRevokingId] = useState('');

  const loadDevices = useCallback(async () => {
    setDevicesLoading(true);
    setDevicesError('');
    try {
      const res = await queryBrowserExtensionDevices();
      setDevices(res.list ?? []);
    } catch (e) {
      setDevicesError(e instanceof Error ? e.message : '扩展设备加载失败');
    } finally {
      setDevicesLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadDevices();
  }, [loadDevices]);

  const connectionInfo = useMemo(
    () => (pairing ? buildBrowserExtensionConnectionInfo(window.location.origin, pairing.code) : ''),
    [pairing],
  );

  const activeCount = devices.filter((row) => row.status === 'active').length;

  const handleGenerate = async () => {
    if (!canManageExtension) {
      message.warning('当前账号没有创建商品草稿的权限');
      return;
    }
    if (remoteHttp) {
      message.warning('远程部署请先启用 HTTPS，再连接浏览器扩展');
      return;
    }
    setGenerating(true);
    try {
      const created = await createBrowserExtensionPairing();
      setPairing(created);
      message.success('一次性连接信息已生成');
    } catch (e) {
      message.error(e instanceof Error ? e.message : '生成连接信息失败');
    } finally {
      setGenerating(false);
    }
  };

  const handleCopy = async () => {
    if (!connectionInfo) return;
    try {
      await copyTextToClipboard(connectionInfo);
      message.success('连接信息已复制，请粘贴到扩展侧边栏');
    } catch {
      message.error('复制失败，请手动复制配对码');
    }
  };

  const handleRevoke = async (row: BrowserExtensionDeviceRow) => {
    setRevokingId(row.id);
    try {
      await revokeBrowserExtensionDevice(row.id);
      message.success('扩展设备已撤销');
      await loadDevices();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '撤销失败');
    } finally {
      setRevokingId('');
    }
  };

  return (
    <TmPageContainer
      title="浏览器扩展采集"
      subTitle="在当前淘宝/天猫页面直接采集；无需 OpenCLI 宿主机进程，也不会额外弹出浏览器。"
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => void loadDevices()} loading={devicesLoading}>
          刷新设备
        </Button>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          type="success"
          showIcon
          icon={<CheckCircleOutlined />}
          message="本地部署与 Docker 使用同一条扩展链路"
          description="扩展访问你当前打开的 TradeMind 地址并主动提交结果，不依赖 host.docker.internal:3100。Playwright Collector 仍保留为后台备用采集引擎。"
        />
        {!canManageExtension ? (
          <Alert
            type="warning"
            showIcon
            message="当前账号不可管理扩展"
            description="可以查看已连接设备，但需要商品写入权限才能生成配对信息或撤销设备。"
          />
        ) : null}
        {remoteHttp ? (
          <Alert
            type="error"
            showIcon
            message="远程 HTTP 地址不能连接扩展"
            description="请先为 TradeMind 配置 HTTPS。本机 localhost / 127.0.0.1 开发地址可以继续使用 HTTP。"
          />
        ) : null}

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={14}>
            <SectionCard
              title="安装与首次连接"
              headerExtra={<Tag color="blue">只需配对一次</Tag>}
              style={{ height: '100%' }}
            >
              <Steps
                direction="vertical"
                size="small"
                current={pairing ? 2 : 1}
                items={[
                  {
                    title: '构建并加载扩展',
                    description: (
                      <Space direction="vertical" size={2}>
                        <Text code>pnpm build:browser-extension</Text>
                        <Text type="secondary">在 Chrome / Edge 扩展管理页加载 browser-extension/dist</Text>
                      </Space>
                    ),
                  },
                  {
                    title: '生成一次性连接信息',
                    description: '有效期 10 分钟，只能使用一次；不会暴露 Admin 登录令牌。',
                  },
                  {
                    title: '粘贴到扩展侧边栏',
                    description: '打开扩展，粘贴连接信息并点击「连接」。',
                  },
                  {
                    title: '在商品页点击采集',
                    description: '扩展会读取当前页面并由 Backend 创建商品草稿。',
                  },
                ]}
              />
              <Space wrap style={{ marginTop: 16 }}>
                <Button
                  type="primary"
                  icon={<KeyOutlined />}
                  onClick={() => void handleGenerate()}
                  loading={generating}
                  disabled={!canManageExtension || remoteHttp}
                >
                  生成连接信息
                </Button>
                {pairing ? (
                  <Button icon={<CopyOutlined />} onClick={() => void handleCopy()}>
                    复制连接信息
                  </Button>
                ) : null}
              </Space>
              {pairing ? (
                <Alert
                  style={{ marginTop: 16 }}
                  type="info"
                  showIcon
                  message={
                    <Space wrap>
                      <Text strong>配对码</Text>
                      <Text code copyable>
                        {pairing.code}
                      </Text>
                    </Space>
                  }
                  description={`请在 ${formatDateTime(pairing.expiresAt)} 前完成连接。优先使用「复制连接信息」，扩展会同时获得当前 TradeMind 地址。`}
                />
              ) : null}
            </SectionCard>
          </Col>
          <Col xs={24} lg={10}>
            <SectionCard title="为什么更省心" style={{ height: '100%' }}>
              <Space direction="vertical" size={14} style={{ width: '100%' }}>
                <div>
                  <Title level={5} style={{ marginBottom: 4 }}>
                    <ChromeOutlined /> 使用当前浏览器
                  </Title>
                  <Text type="secondary" style={{ marginBottom: 0 }}>
                    登录、验证码和页面确认都发生在你正在使用的标签页，不再维护另一套浏览器登录态。
                  </Text>
                </div>
                <div>
                  <Title level={5} style={{ marginBottom: 4 }}>
                    <SafetyOutlined /> 专用设备令牌
                  </Title>
                  <Text type="secondary" style={{ marginBottom: 0 }}>
                    令牌只用于扩展采集，可在本页随时撤销；服务端仅保存令牌哈希。
                  </Text>
                </div>
                <Alert
                  type="warning"
                  showIcon
                  message="当前首版支持淘宝/天猫当前页"
                  description="批量后台任务仍使用 Playwright 或现有 OpenCLI。后续平台通过扩展内置适配器逐步增加，不远程加载脚本。"
                />
              </Space>
            </SectionCard>
          </Col>
        </Row>

        <SectionCard title="已连接设备" description={`当前 ${activeCount} 个有效设备`}>
          {devicesError ? (
            <Alert
              type="error"
              showIcon
              message="设备加载失败"
              description={devicesError}
              action={
                <Button size="small" onClick={() => void loadDevices()}>
                  重试
                </Button>
              }
            />
          ) : devicesLoading ? (
            <Skeleton active paragraph={{ rows: 3 }} />
          ) : devices.length === 0 ? (
            <EmptyState title="还没有连接浏览器扩展" />
          ) : (
            <Table<BrowserExtensionDeviceRow>
              rowKey="id"
              size="small"
              pagination={false}
              scroll={{ x: 720 }}
              dataSource={devices}
              columns={[
                {
                  title: '设备',
                  dataIndex: 'name',
                  width: 220,
                  render: (value: string) => <Text strong>{value}</Text>,
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 100,
                  render: (value: string) => deviceStatusTag(value),
                },
                {
                  title: '最近使用',
                  dataIndex: 'lastUsedAt',
                  width: 180,
                  render: (value?: string | null) => (value ? formatDateTime(value) : '尚未采集'),
                },
                {
                  title: '有效期至',
                  dataIndex: 'expiresAt',
                  width: 180,
                  render: (value: string) => formatDateTime(value),
                },
                {
                  title: '操作',
                  key: 'actions',
                  width: 110,
                  fixed: 'right',
                  render: (_, row) =>
                    row.status === 'active' && canManageExtension ? (
                      <Popconfirm
                        title="撤销这个扩展设备？"
                        description="撤销后该浏览器需要重新配对。"
                        onConfirm={() => void handleRevoke(row)}
                      >
                        <Button
                          danger
                          type="link"
                          size="small"
                          icon={<DeleteOutlined />}
                          loading={revokingId === row.id}
                        >
                          撤销
                        </Button>
                      </Popconfirm>
                    ) : (
                      <Text type="secondary">—</Text>
                    ),
                },
              ]}
            />
          )}
        </SectionCard>
      </Space>
    </TmPageContainer>
  );
}
