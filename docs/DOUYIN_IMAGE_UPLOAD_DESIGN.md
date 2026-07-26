# 抖店图片上传设计

## API

官方接口：`supplyCenter.material.batchUploadImageSync`

支持：公网 URL（SourceURL）或文件字节（Reader + MimeType）

## 幂等缓存（DouyinImageAsset）

```
上传前检查:
  SELECT * FROM douyin_image_assets
  WHERE shop_id = ? AND content_hash = ?
  AND status = 'uploaded'
  → 命中则复用 platform_image_id

上传后:
  → 成功: INSERT/UPDATE status='uploaded', platform_image_id
  → 超时: status='unknown_result', ManualReviewRequired=true
  → 失败: status='failed'
```

幂等 key：`douyin-image-upload:{shopId}:{storageObjectKey}:{contentHash}`

## 内容哈希

SHA256 of image bytes（存储前计算）

## unknown_result 处理

上传后超时：
1. `DouyinImageAsset.Status = unknown_result`
2. 下次相同 content_hash 上传前，先通过 imageId 查询平台确认
3. 人工核查路径：检查抖店素材中心

## 图片类型

| type | 用途 |
|------|------|
| main | 主图 |
| desc | 详情图 |
| sku | SKU 图 |
