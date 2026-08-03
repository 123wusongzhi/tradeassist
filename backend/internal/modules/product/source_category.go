package product

import (
	"encoding/json"
	"strings"
	"unicode"
)

// SourceCategoryFromRaw extracts the best available source category without
// mutating collected raw data. The normalized key is deliberately stable and
// is used only for tenant-owned mapping suggestions.
func SourceCategoryFromRaw(raw json.RawMessage) (string, string) {
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		return "", ""
	}
	for _, key := range []string{"leafCategory", "category", "categoryName", "catName"} {
		if value, ok := root[key]; ok {
			if name, ok := value.(string); ok && strings.TrimSpace(name) != "" {
				return normalizeSourceCategory(name), strings.TrimSpace(name)
			}
		}
	}
	return "", ""
}

func normalizeSourceCategory(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
