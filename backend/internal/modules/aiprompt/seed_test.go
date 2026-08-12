package aiprompt

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureDefaultsMigratesLegacyOzonAttributePromptPolicyV3(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AIPrompt{}))
	legacySystem, legacyUser := legacyBuiltinOzonAttributeSuggestionsV1()
	require.NoError(t, db.Create(&AIPrompt{
		Code: CodeOzonAttributeSuggestions, Name: "legacy", Scene: "product",
		SystemPrompt: legacySystem, UserPrompt: legacyUser, MaxTokens: 2500, Enabled: true,
	}).Error)

	require.NoError(t, EnsureDefaults(t.Context(), db))
	var migrated AIPrompt
	require.NoError(t, db.Where("code = ?", CodeOzonAttributeSuggestions).First(&migrated).Error)
	require.Contains(t, migrated.UserPrompt, "{{context}}")
	require.Contains(t, migrated.UserPrompt, "{{sourceRefs}}")
	require.Contains(t, migrated.UserPrompt, "JSON 字符串")
	require.NotContains(t, migrated.SystemPrompt, "没有证据就省略该属性")
	require.Contains(t, string(migrated.OutputSchema), "sourceRefs")
	require.Contains(t, string(migrated.OutputSchema), "inferenceBasis")
	require.GreaterOrEqual(t, migrated.MaxTokens, 4096)
	var facts AIPrompt
	require.NoError(t, db.Where("code = ?", CodeOzonAttributeFacts).First(&facts).Error)
	require.Contains(t, facts.UserPrompt, "{{context}}")
	require.Contains(t, string(facts.OutputSchema), "facts")
}

func TestEnsureDefaultsMigratesOnlyExactBuiltinOzonAttributePromptV2(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AIPrompt{}))
	v2System, v2User, v2Schema := builtinOzonAttributeSuggestionsV2()
	require.NoError(t, db.Create(&AIPrompt{
		Code: CodeOzonAttributeSuggestions, Name: "builtin-v2", Scene: "product",
		SystemPrompt: v2System, UserPrompt: v2User, OutputSchema: v2Schema,
		MaxTokens: 4096, Enabled: true,
	}).Error)

	require.NoError(t, EnsureDefaults(t.Context(), db))
	var migrated AIPrompt
	require.NoError(t, db.Where("code = ?", CodeOzonAttributeSuggestions).First(&migrated).Error)
	require.Contains(t, migrated.SystemPrompt, "policy v3")
	require.Contains(t, migrated.UserPrompt, "inferenceBasis")
	require.Contains(t, string(migrated.OutputSchema), "factRefs")
}

func TestEnsureDefaultsPreservesCustomizedOzonAttributePrompt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AIPrompt{}))
	require.NoError(t, db.Create(&AIPrompt{
		Code: CodeOzonAttributeSuggestions, Name: "custom", Scene: "product",
		SystemPrompt: "custom system", UserPrompt: "custom {{attributes}}", MaxTokens: 900, Enabled: true,
	}).Error)

	require.NoError(t, EnsureDefaults(t.Context(), db))
	var preserved AIPrompt
	require.NoError(t, db.Where("code = ?", CodeOzonAttributeSuggestions).First(&preserved).Error)
	require.Equal(t, "custom system", preserved.SystemPrompt)
	require.Equal(t, "custom {{attributes}}", preserved.UserPrompt)
	require.Equal(t, 900, preserved.MaxTokens)
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), `["true"]`)
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "主 SKU")
	require.Contains(t, OzonAttributeFactsRuntimePolicy(), "全部代表 SKU")
	require.Contains(t, OzonAttributeFactsRuntimePolicy(), "顶层只能有 facts")
	require.Contains(t, OzonAttributeFactsRuntimePolicy(), "timeout")
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "context.skuVariations")
	require.Contains(t, OzonAttributeFactsRuntimePolicy(), "context.skuVariations")
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "semanticHint")
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "28 天")
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "672")
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "4G 全网通")
	require.Contains(t, OzonAttributeSuggestionRuntimePolicy(), "Series 30+")
}
