package ozon

import (
	"context"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type ozonProvider struct{}

// NewProvider constructs the Ozon Seller API platform integration (beta).
func NewProvider() platformp.Provider { return ozonProvider{} }

func (ozonProvider) Platform() string { return PlatformID }

func (ozonProvider) Name() string { return "Ozon" }

func (ozonProvider) Status() string { return platformp.StatusBeta }

func (ozonProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{
		platformp.CapProductPublish,
		platformp.CapShopInfo,
	}
}

func (ozonProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "api_key",
		Fields: []platformp.AuthField{
			{
				Name:      "appKey",
				Label:     "Client ID",
				Type:      "text",
				Required:  true,
				Sensitive: false,
				Hint:      "Ozon 卖家后台 → 设置 → API Keys 页面中的 Client-ID（数字）",
			},
			{
				Name:      "accessToken",
				Label:     "Api-Key",
				Type:      "password",
				Required:  true,
				Sensitive: true,
				Hint:      "Seller API 密钥，生成时只显示一次；凭据将加密存储",
			},
		},
	}
}

func (ozonProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	// Ozon 凭证为店铺级（Client-ID + Api-Key），无需部署级应用配置。
	return platformp.PlatformAppConfigSchema{}
}

func (ozonProvider) PublishConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PublishConfigPresetForPlatform(PlatformID)
}

func (ozonProvider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	cfg, err := ResolveRuntime(req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client := newClient(cfg)
	info, err := client.getSellerInfo(cctx)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	shopName := strings.TrimSpace(info.Company.Name)
	currency := strings.TrimSpace(info.Company.Currency)
	country := strings.TrimSpace(info.Company.Country)
	return &platformp.TestConnectionResult{
		OK:             true,
		Message:        "ozon connection ok",
		ShopName:       shopName,
		ExternalShopID: cfg.ClientID,
		Region:         country,
		Currency:       currency,
	}, nil
}
