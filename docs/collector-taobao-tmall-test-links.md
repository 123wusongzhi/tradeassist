# 淘宝/天猫采集器测试链接验收表

> 状态：**已可用（持续真实链接回归）**。每轮验收必须记录实际采集引擎；OpenCLI
> 使用扩展连接的宿主机 Chrome，Playwright 使用独立 `taobao_tmall` 浏览器 Profile。

## 验收说明

| 字段 | 说明 |
| --- | --- |
| 需要登录 | 未登录时是否出现 `LOGIN_REQUIRED` |
| 标题 | 是否采到非空标题 |
| 价格 | 是否识别价格（缺失应有 `PRICE_NOT_FOUND` warning） |
| 主图数 | 主图数量（0 则任务应失败 `MAIN_IMAGES_EMPTY`） |
| 详情图数 | 详情图数量（0 可有 `DETAIL_IMAGES_INCOMPLETE` warning） |
| SKU 数 | 规格行数（不完整可有 `SKU_INCOMPLETE` warning） |
| warning | 采集 warning 码 |
| error | 失败 error 码 |
| 草稿 | 是否成功创建商品草稿 |

## 每轮验收元数据

复制本表记录每一轮，不要只写“浏览器能打开”：

| 项目 | 实测值 |
| --- | --- |
| 日期 / 操作人 | 待填 |
| 部署方式 | 本地 / Docker |
| 期望引擎 | OpenCLI / Playwright |
| 任务列表显示的实际引擎 | 待填 |
| 引擎状态 | `enabled` / `configured` / `reachable` / `ready` |
| Playwright Collector `3001/health` | 待填 |
| OpenCLI Bridge `3100/health`（使用时） | 待填 |
| Chrome / Profile 登录与验证状态 | 待填 |
| 适配器版本或同步结果 | 待填 |

验收顺序：

1. 先用当前主引擎执行下面的真实链接字段验收。启用 OpenCLI 且默认值为
   `opencli` 时，必须确认任务“实际引擎”为 OpenCLI。
2. 再用 Playwright 至少回归普通淘宝、普通天猫和多 SKU 各一条，确认备用引擎可用。
3. 停止 OpenCLI Bridge，确认 OpenCLI 任务明确失败，同时 Playwright 任务仍能访问
   `3001`；重新启动后确认状态恢复。

完整部署与故障隔离说明见
[采集引擎与部署指南](collector-engines.md)。

## 已验收记录

| 日期 / 操作人 | 部署方式 | 期望引擎 | 实际引擎 | 引擎状态 | 适配器版本/同步 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-07-31 / Codex | 本地 | OpenCLI | OpenCLI | enabled / configured / reachable / ready | 仓库 `collector/opencli-adapters/tmall` 同步后运行 | SKU 价格探测 `sku-price-max=24`；采集过程中触发过一次滑块验证，人工完成后恢复 |

## 普通淘宝商品（5 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 2 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 3 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 4 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 5 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |

## 普通天猫商品（5 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 6 | `https://detail.tmall.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 7 | `https://detail.tmall.com/item.htm?id=622919682584` | 是 | 恩爵薄款固态继电器模组小型导轨式SSK10D带底座5v24v直流控交流 | 8.6（per-SKU 4–191） | 6 | 21 | 16 | — | — | 待测 |
| 8 | `https://detail.tmall.hk/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 9 | `https://chaoshi.tmall.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 10 | `https://ju.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |

## 有 SKU 商品（5 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 11 | `https://item.taobao.com/item.htm?id=【多规格淘宝】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 12 | `https://detail.tmall.com/item.htm?id=997134693410` | 是 | 老年犬专用站立辅助趴趴凳防滑关节护理瘫痪狗狗辅助起身保暖狗垫 | 351.8（per-SKU 351.8–900.8） | 6 | 8 | 56（8 色 × 7 码） | — | — | 待测 |
| 13 | `https://item.taobao.com/item.htm?id=【颜色+尺码】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 14 | `https://detail.tmall.com/item.htm?id=【颜色+尺码】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 15 | `https://world.taobao.com/item/【填写】.htm` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |

## 多主图商品（2 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 16 | `https://item.taobao.com/item.htm?id=【多主图】` | 待测 | 待测 | 待测 | ≥2 | 待测 | 待测 | — | — | 待测 |
| 17 | `https://detail.tmall.com/item.htm?id=【多主图】` | 待测 | 待测 | 待测 | ≥2 | 待测 | 待测 | — | — | 待测 |

## 多详情图商品（2 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 18 | `https://item.taobao.com/item.htm?id=【长详情】` | 待测 | 待测 | 待测 | 待测 | ≥3 | 待测 | — | — | 待测 |
| 19 | `https://detail.tmall.com/item.htm?id=【长详情】` | 待测 | 待测 | 待测 | 待测 | ≥3 | 待测 | — | — | 待测 |

## 已下架 / 异常商品（1 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 20 | `https://item.taobao.com/item.htm?id=【已下架】` | — | — | — | 0 | 0 | 0 | — | `ITEM_NOT_FOUND` | 否 |

## 不支持链接（回归）

| 场景 | 链接示例 | 期望 error |
| --- | --- | --- |
| 淘宝首页 | `https://www.taobao.com/` | `UNSUPPORTED_TAOBAO_URL` |
| 店铺页 | `https://shop.taobao.com/shop/view_shop.htm?shop_id=xxx` | `UNSUPPORTED_TAOBAO_URL` |
| 搜索页 | `https://s.taobao.com/search?q=xxx` | `UNSUPPORTED_TAOBAO_URL` |

## 维持「已可用」门槛

持续满足以下全部条件，才能维持采集中心 **已可用** 状态：

1. 上表 20 条真实链接由当前主引擎完成验收，成功率稳定（建议 ≥80% 普通商品可创建草稿）。
2. Playwright 备用引擎至少完成普通淘宝、普通天猫和多 SKU 三类冒烟回归。
3. 每条任务的实际引擎可见，重试不改变引擎，OpenCLI 失败不静默切换 Playwright。
4. Bridge 停止只影响 OpenCLI；Playwright、1688 与 backend 健康状态不受影响。
5. 登录 / 验证 / 下架三类异常提示准确可读。
6. `EMPTY_RESULT` 归类为可恢复解析失败；只有明确 `ITEM_NOT_FOUND` 才判商品不存在。
7. 主图为空必失败、价格缺失 warning + 发布前拦截生效。
8. 失败任务中心可重试，并能按所选引擎给出正确处理入口。
9. 无绕过验证码相关逻辑。
