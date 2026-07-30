# CORS 生产配置指南（P2）

> staging / production 必须显式配置 CORS 白名单；development 默认放行 localhost。

## 环境变量

| 变量 | 说明 | 默认 |
| --- | --- | --- |
| `CORS_ALLOWED_ORIGINS` | 逗号分隔 Origin 列表，**staging/production 必填** | 空 |
| `CORS_ALLOWED_METHODS` | 允许方法 | `GET, POST, PUT, PATCH, DELETE, OPTIONS` |
| `CORS_ALLOWED_HEADERS` | 允许请求头 | `Authorization, Content-Type, X-Request-Id` |
| `CORS_EXPOSED_HEADERS` | 暴露响应头 | `X-Request-Id` |
| `CORS_ALLOW_CREDENTIALS` | `Access-Control-Allow-Credentials` | `true` |
| `CORS_MAX_AGE` | 预检缓存秒数 | `43200`（12h） |

解析：`config.Load()` → `splitCSV`；中间件 `middleware.CORS`。

## 行为规则

### Origin 校验

1. 无 `Origin` 头（同源、curl）→ 直接 `Next()`，不附加 CORS 头。
2. 白名单精确匹配（忽略尾部 `/`，大小写不敏感）。
3. **禁止** staging/production 使用 `*`（即使未配 credentials）。
4. `*` + `CORS_ALLOW_CREDENTIALS=true` → 启动 `Validate()` 失败。

### Development / Demo

未列入白名单的 `http(s)://localhost:*` 与 `127.0.0.1` **自动放行**，便于 Vite  dev server。

### 预检与 Fail-Fast

合法 Origin → OPTIONS `204`；非法 → `403`。staging/production 未配 `CORS_ALLOWED_ORIGINS` 启动失败（`CONFIG_REQUIRED`）。

## 配置示例

### 本地开发（`.env.example`）

```env
# development 可不设；localhost 自动放行
# CORS_ALLOWED_ORIGINS=http://localhost:8000
```

### 预发

```env
APP_ENV=staging
ADMIN_PUBLIC_URL=https://admin.staging.example.com
CORS_ALLOWED_ORIGINS=https://admin.staging.example.com
CORS_ALLOW_CREDENTIALS=true
```

### 生产（`.env.production.example`）

```env
APP_ENV=production
ADMIN_PUBLIC_URL=https://admin.example.com
API_PUBLIC_URL=https://api.example.com
CORS_ALLOWED_ORIGINS=https://admin.example.com
```

多前端域：

```env
CORS_ALLOWED_ORIGINS=https://admin.example.com,https://ops.example.com
```

## 注意

- `ADMIN_PUBLIC_URL` / `API_PUBLIC_URL` 应与 CORS Origin 同域；Nginx 与 API 白名单保持一致。
- production 禁止 `*`；credentials=true 须具体域名。详见 `ENVIRONMENT_PROFILE_GUIDE.md`、`cors_test.go`。
