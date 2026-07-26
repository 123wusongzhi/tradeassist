package operationtask

import (
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
)

const redactedFieldMarker = "redactedSensitiveField"

func redactSafeJSON(raw datatypes.JSON) datatypes.JSON {
	if len(raw) == 0 || !json.Valid(raw) {
		return datatypes.JSON([]byte(`{}`))
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return datatypes.JSON([]byte(`{}`))
	}
	redacted := redactSafeValue(value)
	data, err := json.Marshal(redacted)
	if err != nil {
		return datatypes.JSON([]byte(`{}`))
	}
	out := datatypes.JSON(data)
	if len(out) > 8192 || payloadHasSecret(out) {
		return datatypes.JSON([]byte(`{}`))
	}
	return out
}

func redactSafeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		redactedCount := 0
		for key, child := range typed {
			if sensitivePayloadKey(key) {
				redactedCount++
				continue
			}
			out[key] = redactSafeValue(child)
		}
		if redactedCount > 0 {
			out[redactedFieldMarker] = redactedCount
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for i, child := range typed {
			if i >= 50 {
				out = append(out, "...truncated")
				break
			}
			out = append(out, redactSafeValue(child))
		}
		return out
	case string:
		if safeTextHasSecret(typed) || urlTextHasSecret(typed) {
			return "[redacted sensitive content]"
		}
		if len(typed) > 500 {
			return typed[:500] + "..."
		}
		return typed
	default:
		return value
	}
}

func urlTextHasSecret(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "://") && (strings.Contains(lower, "@") || strings.Contains(lower, "token=") || strings.Contains(lower, "access_token=") || strings.Contains(lower, "password="))
}
