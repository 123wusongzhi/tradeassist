package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func newBrowserExtensionTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:browser_extension_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CollectTask{}, &CollectTaskEvent{}))
	return db
}

func newBrowserExtensionProductDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:browser_extension_product_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&product.Product{}, &product.ProductImage{}, &product.ProductSKU{}))
	return db
}

func browserExtensionProductJSON(title string) json.RawMessage {
	price := 380.8
	stock := 7
	payload := map[string]any{
		"source":    "taobao_tmall",
		"sourceUrl": "https://detail.tmall.com/item.htm?id=997134693410",
		"title":     title,
		"currency":  "CNY",
		"mainImages": []string{
			"https://img.alicdn.com/imgextra/i3/2214855611/O1CN01uutrBS1rJtRSQJDwV_!!4611686018427380667-0-item_pic.jpg",
		},
		"descriptionImages": []string{},
		"attributes":        map[string]any{"品牌": "测试"},
		"skus": []map[string]any{
			{
				"properties":    map[string]string{"颜色分类": "红色"},
				"price":         price,
				"originalPrice": 478.8,
				"stock":         stock,
				"stockStatus":   "有货",
				"logisticsTime": "预计明天发货",
				"skuCode":       "6142861994389",
				"image":         "https://img.alicdn.com/imgextra/i1/2214855611/O1CN01Zc8VnU1rJtPEbKhAU_!!2214855611.jpg",
				"raw":           map[string]any{"source": "skuBase"},
			},
		},
		"raw": map[string]any{
			"provider":           "browser_extension",
			"productPrice":       380.8,
			"qualityWarnings":    []string{},
			"skuPriceProbeCount": 1,
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func newBrowserExtensionService(t *testing.T) *Service {
	t.Helper()
	return &Service{
		DB:       newBrowserExtensionTaskTestDB(t),
		Products: &product.Service{DB: newBrowserExtensionProductDB(t)},
	}
}

func TestCreateBrowserExtensionTaskPersistsRunningBoundTask(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	out, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "taobao_tmall",
		URL:      "https://detail.tmall.com/item.htm?id=997134693410",
	})
	require.NoError(t, err)
	require.Equal(t, StatusRunning, out.Status)
	require.Equal(t, "taobao_tmall", out.Source)

	var task CollectTask
	require.NoError(t, svc.DB.First(&task, "id = ?", out.ID).Error)
	require.True(t, browserExtensionTaskBoundTo(&task, deviceID))
	require.False(t, browserExtensionTaskBoundTo(&task, uuid.New()))
}

func TestCreateBrowserExtensionTaskRejectsUnsupportedSourceOrURL(t *testing.T) {
	svc := newBrowserExtensionService(t)
	_, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  uuid.New(),
		DeviceID: uuid.New(),
		Source:   "custom",
		URL:      "https://detail.1688.com/offer/1.html",
	})
	require.Error(t, err)

	_, err = svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  uuid.New(),
		DeviceID: uuid.New(),
		Source:   "taobao_tmall",
		URL:      "https://example.com/not-a-product",
	})
	require.Error(t, err)

	_, err = svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  uuid.New(),
		DeviceID: uuid.New(),
		Source:   "1688",
		URL:      "https://detail.tmall.com/item.htm?id=1",
	})
	require.Error(t, err)

	_, err = svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  uuid.New(),
		DeviceID: uuid.New(),
		Source:   "1688",
		URL:      "https://www.1688.com/",
	})
	require.Error(t, err)
}

func TestCreateBrowserExtensionTaskAccepts1688OfferURL(t *testing.T) {
	svc := newBrowserExtensionService(t)
	out, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  uuid.New(),
		DeviceID: uuid.New(),
		Source:   "1688",
		URL:      "https://detail.1688.com/offer/1054514049952.html",
	})
	require.NoError(t, err)
	require.Equal(t, StatusRunning, out.Status)
	require.Equal(t, "1688", out.Source)
}

func browserExtension1688ProductJSON(title string) json.RawMessage {
	price := 12.5
	stock := 99
	payload := map[string]any{
		"source":    "1688",
		"sourceUrl": "https://detail.1688.com/offer/1054514049952.html",
		"title":     title,
		"currency":  "CNY",
		"mainImages": []string{
			"https://cbu01.alicdn.com/img/ibank/O1CN01test.jpg",
		},
		"descriptionImages": []string{},
		"attributes": map[string]any{
			"单位":    "个",
			"最小起订量": 2,
		},
		"skus": []map[string]any{
			{
				"properties": map[string]string{"颜色": "黑色"},
				"price":      price,
				"stock":      stock,
				"skuCode":    "sku-black",
				"image":      "https://cbu01.alicdn.com/img/ibank/O1CN01black.jpg",
				"raw":        map[string]any{"source": "skuMap"},
			},
		},
		"raw": map[string]any{
			"provider":     "browser_extension",
			"productPrice": 12.5,
			"priceTiers": []map[string]any{
				{"beginAmount": 2, "price": 12.5},
				{"beginAmount": 100, "price": 11.8},
			},
			"priceTierWarning": "阶梯价已完整保存在 raw.priceTiers；SKU.price 为首档/SKU 单价，非唯一成交价",
			"minOrderQuantity": 2,
			"unit":             "个",
			"qualityWarnings":  []string{},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func TestCompleteBrowserExtensionTaskImports1688Draft(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	created, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "1688",
		URL:      "https://detail.1688.com/offer/1054514049952.html",
	})
	require.NoError(t, err)

	done, err := svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: browserExtension1688ProductJSON("1688 扩展采集测试商品"),
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, done.Status)
	require.NotNil(t, done.ResultProductID)

	var draft product.Product
	require.NoError(t, svc.Products.DB.First(&draft, "id = ?", done.ResultProductID).Error)
	require.Equal(t, int64(11), draft.TenantID)
	require.Equal(t, "1688", draft.Source)
	require.Contains(t, string(draft.RawData), "priceTiers")
	require.Contains(t, string(draft.RawData), "最小起订量")
}

func TestCompleteBrowserExtensionTaskImportsDraftAndSucceeds(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	created, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "taobao_tmall",
		URL:      "https://detail.tmall.com/item.htm?id=997134693410",
	})
	require.NoError(t, err)

	done, err := svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: browserExtensionProductJSON("浏览器扩展采集测试商品"),
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, done.Status)
	require.NotNil(t, done.ResultProductID)

	var persisted CollectTask
	require.NoError(t, svc.DB.First(&persisted, "id = ?", created.ID).Error)
	require.Equal(t, StatusSuccess, persisted.Status)
	require.Equal(t, *done.ResultProductID, *persisted.ResultProductID)

	var draft product.Product
	require.NoError(t, svc.Products.DB.First(&draft, "id = ?", done.ResultProductID).Error)
	require.Equal(t, int64(11), draft.TenantID)
}

func TestCompleteBrowserExtensionTaskRejectsWrongDevice(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	created, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "taobao_tmall",
		URL:      "https://detail.tmall.com/item.htm?id=997134693410",
	})
	require.NoError(t, err)

	_, err = svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    uuid.New(),
		TaskID:      created.ID,
		ProductJSON: browserExtensionProductJSON("其他设备"),
	})
	require.ErrorIs(t, err, ErrBrowserExtensionTaskInvalid)
}

func TestCompleteBrowserExtensionTaskCannotBeSubmittedTwice(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	created, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "taobao_tmall",
		URL:      "https://detail.tmall.com/item.htm?id=997134693410",
	})
	require.NoError(t, err)

	_, err = svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: browserExtensionProductJSON("第一次提交"),
	})
	require.NoError(t, err)

	_, err = svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: browserExtensionProductJSON("第二次提交"),
	})
	require.ErrorIs(t, err, ErrBrowserExtensionTaskNotActive)
}

func TestFailBrowserExtensionTaskMarksFailed(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	created, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "taobao_tmall",
		URL:      "https://detail.tmall.com/item.htm?id=997134693410",
	})
	require.NoError(t, err)

	failed, err := svc.FailBrowserExtensionTask(context.Background(), BrowserExtensionFailureInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		TaskID:   created.ID,
		Code:     "VERIFY_REQUIRED",
		Message:  "页面需要安全验证",
	})
	require.NoError(t, err)
	require.Equal(t, StatusFailed, failed.Status)

	var task CollectTask
	require.NoError(t, svc.DB.First(&task, "id = ?", created.ID).Error)
	require.Equal(t, StatusFailed, task.Status)
	require.Contains(t, task.ErrorMessage, "VERIFY_REQUIRED")

	_, err = svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: browserExtensionProductJSON("失败后再提交"),
	})
	require.ErrorIs(t, err, ErrBrowserExtensionTaskNotActive)
}

func TestCompleteBrowserExtensionTaskWithEmptyProductReleasesLock(t *testing.T) {
	svc := newBrowserExtensionService(t)
	adminID := uuid.New()
	deviceID := uuid.New()
	created, err := svc.CreateBrowserExtensionTask(context.Background(), BrowserExtensionTaskInput{
		TenantID: 11,
		AdminID:  adminID,
		DeviceID: deviceID,
		Source:   "taobao_tmall",
		URL:      "https://detail.tmall.com/item.htm?id=997134693410",
	})
	require.NoError(t, err)

	_, err = svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: json.RawMessage(`{}`),
	})
	require.Error(t, err)

	// 失败后锁释放，同一设备可以重试成功。
	done, err := svc.CompleteBrowserExtensionTask(context.Background(), BrowserExtensionResultInput{
		TenantID:    11,
		AdminID:     adminID,
		DeviceID:    deviceID,
		TaskID:      created.ID,
		ProductJSON: browserExtensionProductJSON("重试成功"),
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, done.Status)
}
