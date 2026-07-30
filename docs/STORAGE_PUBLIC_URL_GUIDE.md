# Storage 公网 URL 配置指南（H1.3）

> 抖店图片上传、平台刊登外链图片均需要**公网 HTTPS** 可访问的图片 URL。

## 配置字段

| 设置键 | 说明 |
| --- | --- |
| `storage.public_base` | 通用公网前缀（推荐） |
| `s3_public_base` / `cos_public_base` / `oss_public_base` | 云存储专用前缀 |

后端解析：`backend/internal/pkg/storagepublic/public_base.go` → `ResolvePublicBase()`

## 本地存储注意

- 默认 `local` + `/static` **不能**通过抖店图片上传前置检查
- 必须配置可被外网访问的 HTTPS 域名，例如 `https://cdn.example.com/uploads`

## 配置步骤

1. 打开 **设置 → 存储**
2. 填写 `public_base`（完整 HTTPS 前缀，无尾部斜杠冲突）
3. 点击 **测试公网访问**（上传探测图 → 匿名 GET 验证）
4. 在 **设置 → 配置状态** 确认「Storage 公网访问」为已配置

## 缺失提示（统一文案）

> 当前 Storage 尚未配置公网访问地址。抖店图片上传前需要确保商品图片可通过公网 URL 访问。

## 与 AI 图片的关系

- 处理结果写入 Storage 后，若 `public_base` 缺失，失败码：`storage_public_url_missing`
- 失败任务中心分类：`ai_image_storage_public_url_missing`

## 相关入口

- `/settings/storage`
- `/settings/config-status`（`storage_public_access` 卡片）
- `/settings/platforms` → 抖店生产预检 → `storage.public_access`

## 安全

- 测试不泄露内部磁盘路径
- 日志不输出完整密钥
