package inventorysyncp9

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safefields"
	"gorm.io/datatypes"
)

var inventorySyncAllowedMetadataKeys = map[string]bool{
	"platform":             true,
	"providerMode":         true,
	"fixtureScenario":      true,
	"runStatusBefore":      true,
	"runStatusAfter":       true,
	"pageNumber":           true,
	"pageItemCount":        true,
	"totalRecordCount":     true,
	"matchedRecordCount":   true,
	"unmatchedRecordCount": true,
	"conflictRecordCount":  true,
	"failedRecordCount":    true,
	"bindingStatusBefore":  true,
	"bindingStatusAfter":   true,
	"bindingSource":        true,
	"matchStrategy":        true,
	"confidence":           true,
	"calibrationVersion":   true,
	"normalizationVersion": true,
	"reasonCodes":          true,
	"retryable":            true,
	"failureCategory":      true,
	"capabilityBlocked":    true,
	"externalProductId":    true,
	"externalSkuId":        true,
	"localSkuId":           true,
	"payloadHash":          true,
	"fixtureHash":          true,
	"cursorHash":           true,
	"safeMessage":          true,
	"errorCode":            true,
	"stage":                true,
	"requestId":            true,
	"pagesProcessed":       true,
}

func safeInventorySyncMetadataJSON(meta map[string]any) (datatypes.JSON, error) {
	if len(meta) == 0 {
		return datatypes.JSON([]byte(`{}`)), nil
	}
	allowed := map[string]any{}
	for key, value := range meta {
		key = strings.TrimSpace(key)
		if inventorySyncAllowedMetadataKeys[key] {
			allowed[key] = safefields.RedactValue(value)
		}
	}
	encoded, err := json.Marshal(allowed)
	if err != nil {
		return nil, ErrValidation
	}
	return normalizeModelJSON(datatypes.JSON(encoded), maxSafeJSONBytes)
}

func safeProviderMetadataJSON(meta map[string]string) (datatypes.JSON, error) {
	if len(meta) == 0 {
		return datatypes.JSON([]byte(`{}`)), nil
	}
	allowed := map[string]any{}
	for key, value := range meta {
		key = strings.TrimSpace(key)
		if inventorySyncAllowedMetadataKeys[key] {
			allowed[key] = safefields.RedactString(value)
		}
	}
	encoded, err := json.Marshal(allowed)
	if err != nil {
		return nil, ErrProviderPageInvalid
	}
	out, err := normalizeModelJSON(datatypes.JSON(encoded), maxSafeJSONBytes)
	if err != nil {
		return nil, ErrProviderPageInvalid
	}
	return out, nil
}

func safeManualBindingComment(comment string) string {
	return safefields.RedactString(comment)
}

func safeCursorHash(cursor datatypes.JSON) string {
	raw := strings.TrimSpace(string(cursor))
	if raw == "" || raw == "null" || raw == "{}" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
