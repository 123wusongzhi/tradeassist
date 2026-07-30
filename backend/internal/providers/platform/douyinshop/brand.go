package douyinshop

// BrandListStatus explicitly describes the brand list capability status for P3.
// Brand list API requires additional contract verification with Douyin Open Platform.
// This is NOT a bug — it is a deliberate placeholder pending real credential verification.
//
// Product publish uses standard_brand_id from category mapping (already implemented).
const (
	BrandListStatus         = "blocked_by_contract_verification"
	BrandListBlockedMessage = "抖店品牌列表接口需通过合同核查后方可调用；当前阶段使用类目映射中的 standard_brand_id"
)

// BrandListUnsupportedError returns the standard error for brand list attempts.
func BrandListUnsupportedError() *Error {
	e := NewError(CodeDouyinContractMismatch,
		BrandListBlockedMessage,
		"", "blocked_by_contract_verification", "")
	e.Retryable = false
	return e
}

// GetBrandList is explicitly unsupported in P3.
// Returns CodeDouyinContractMismatch with explanation.
// Callers should use standard_brand_id from category attribute mappings.
func (c *Client) GetBrandList() ([]map[string]any, error) {
	return nil, BrandListUnsupportedError()
}
