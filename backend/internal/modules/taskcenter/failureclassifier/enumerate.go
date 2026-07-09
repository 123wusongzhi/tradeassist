package failureclassifier

// AllCategories lists supported failure_category values (API enums).
func AllCategories() []string {
	return []string{
		CategoryPlatformAuth,
		CategoryPlatformPermission,
		CategoryPlatformRateLimit,
		CategoryPlatformAPIError,
		CategoryPlatformConfigIncomplete,
		CategoryNetworkTimeout,
		CategoryCollectorBlocked,
		CategoryCollectorPlatformLogin,
		CategoryCollectorMissingImages,
		CategoryCollectorMissingPrice,
		CategoryCollectorEvaluateScript,
		CategoryCollectorInvalidURL,
		CategoryAIProviderError,
		CategoryAIConfigIncomplete,
		CategoryImageProviderError,
		CategoryStorageError,
		CategoryValidationError,
		CategoryInventoryMappingMissing,
		CategorySKUMappingMissing,
		CategoryWorkerLeaseExpired,
		CategorySystemError,
		CategoryUnknown,
		// AI product text batch review (aiproducttext module)
		"ai_text_generation_failed",
		"ai_text_apply_conflict",
		"ai_text_apply_failed",
		"ai_text_undo_failed",
		"ai_text_quality_warning",
		// AI product image batch review (aiproductimage module)
		"ai_image_process_failed",
		"ai_image_apply_conflict",
		"ai_image_apply_failed",
		"ai_image_undo_failed",
		"ai_image_quality_warning",
		"ai_image_provider_config_missing",
		"ai_image_dashscope_key_missing",
		"ai_image_storage_public_url_missing",
		"ai_image_download_failed",
		"ai_image_unsupported_operation",
	}
}

// AllSeverities lists severity labels.
func AllSeverities() []string {
	return []string{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}
}
