package ozon

import "context"

const pathSellerInfo = "/v1/seller/info"

type sellerInfo struct {
	Company struct {
		Name      string `json:"name"`
		LegalName string `json:"legal_name"`
		Country   string `json:"country"`
		Currency  string `json:"currency"`
		INN       string `json:"inn"`
	} `json:"company"`
	Subscription struct {
		IsPremium bool   `json:"is_premium"`
		Type      string `json:"type"`
	} `json:"subscription"`
}

func (c *ozonClient) getSellerInfo(ctx context.Context) (*sellerInfo, error) {
	var out sellerInfo
	if err := c.postJSON(ctx, pathSellerInfo, map[string]any{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
