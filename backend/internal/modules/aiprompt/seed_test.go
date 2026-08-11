package aiprompt

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureDefaultsMigratesLegacyOzonAttributePromptPolicyV2(t *testing.T) {
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
	require.NotContains(t, migrated.SystemPrompt, "没有证据就省略该属性")
	require.Contains(t, string(migrated.OutputSchema), "sourceRefs")
	require.GreaterOrEqual(t, migrated.MaxTokens, 4096)
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
}
