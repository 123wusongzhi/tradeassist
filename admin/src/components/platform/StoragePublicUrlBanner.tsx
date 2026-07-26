import { history } from '@umijs/max';
import { Alert, Button, Space } from 'antd';

type Props = {
  missing?: boolean;
  localOnly?: boolean;
  compact?: boolean;
};

/** Storage public_base 缺失或本地路径不可外网访问时的提示 */
export default function StoragePublicUrlBanner({
  missing = true,
  localOnly = false,
  compact = false,
}: Props) {
  if (!missing && !localOnly) return null;

  const message = localOnly
    ? '当前存储使用本地路径，抖店图片上传前需要确保商品图片可通过公网 HTTPS 地址访问。'
    : '当前存储尚未配置公网访问地址。抖店图片上传前需要确保商品图片可通过公网地址访问。';

  return (
    <Alert
      type="warning"
      showIcon
      style={{ marginBottom: compact ? 8 : 16 }}
      message={message}
      description={
        compact ? undefined : (
          <Space wrap style={{ marginTop: 8 }}>
            <Button size="small" type="primary" onClick={() => history.push('/settings/storage')}>
              去配置存储
            </Button>
            <Button
              size="small"
              onClick={() => window.open('/docs/STORAGE_PUBLIC_URL_GUIDE.md', '_blank')}
            >
              查看公网访问说明
            </Button>
          </Space>
        )
      }
    />
  );
}
