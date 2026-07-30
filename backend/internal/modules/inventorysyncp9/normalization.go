package inventorysyncp9

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	NormalizationVersionV1 = "sku-normalization-v1"
	MaxSKUIdentifierBytes  = 255
	MaxBarcodeBytes        = 128
)

type NormalizedIdentifier struct {
	OriginalValue        string   `json:"originalValue"`
	NormalizedValue      string   `json:"normalizedValue"`
	NormalizationVersion string   `json:"normalizationVersion"`
	AppliedRules         []string `json:"appliedRules"`
	Valid                bool     `json:"valid"`
	InvalidReasonCode    string   `json:"invalidReasonCode,omitempty"`
}

type SKUIdentifierNormalizer interface {
	NormalizeSKUCode(value string) NormalizedIdentifier
	NormalizeBarcode(value string) NormalizedIdentifier
}

type DefaultSKUIdentifierNormalizer struct{}

func NewDefaultSKUIdentifierNormalizer() DefaultSKUIdentifierNormalizer {
	return DefaultSKUIdentifierNormalizer{}
}

func (DefaultSKUIdentifierNormalizer) NormalizeSKUCode(value string) NormalizedIdentifier {
	result := NormalizedIdentifier{OriginalValue: value, NormalizationVersion: NormalizationVersionV1, Valid: true}
	if len(value) > MaxSKUIdentifierBytes {
		return invalidIdentifier(value, ErrCodeInvalidIdentifier, []string{"max_length"})
	}
	if hasControlRune(value) {
		return invalidIdentifier(value, ErrCodeInvalidIdentifier, []string{"reject_control_characters"})
	}
	normalized := norm.NFKC.String(value)
	result.AppliedRules = append(result.AppliedRules, "unicode_nfkc")
	trimmed := strings.TrimSpace(normalized)
	if trimmed != normalized {
		result.AppliedRules = append(result.AppliedRules, "trim_space")
	}
	normalized = collapseWhitespace(trimmed)
	if normalized != trimmed {
		result.AppliedRules = append(result.AppliedRules, "collapse_whitespace")
	}
	dashed := normalizeDashForms(normalized)
	if dashed != normalized {
		result.AppliedRules = append(result.AppliedRules, "normalize_dash_forms")
	}
	normalized = strings.ToUpper(dashed)
	if normalized != dashed {
		result.AppliedRules = append(result.AppliedRules, "uppercase")
	}
	result.NormalizedValue = normalized
	result.Valid = normalized != ""
	if !result.Valid {
		result.InvalidReasonCode = ReasonMissingExternalIdentifier
	}
	return result
}

func (DefaultSKUIdentifierNormalizer) NormalizeBarcode(value string) NormalizedIdentifier {
	result := NormalizedIdentifier{OriginalValue: value, NormalizationVersion: NormalizationVersionV1, Valid: true}
	if len(value) > MaxBarcodeBytes {
		return invalidIdentifier(value, ErrCodeInvalidIdentifier, []string{"max_length"})
	}
	if hasControlRune(value) {
		return invalidIdentifier(value, ErrCodeInvalidIdentifier, []string{"reject_control_characters"})
	}
	normalized := norm.NFKC.String(value)
	result.AppliedRules = append(result.AppliedRules, "unicode_nfkc")
	trimmed := strings.TrimSpace(normalized)
	if trimmed != normalized {
		result.AppliedRules = append(result.AppliedRules, "trim_space")
	}
	normalized = collapseWhitespace(trimmed)
	if normalized != trimmed {
		result.AppliedRules = append(result.AppliedRules, "collapse_whitespace")
	}
	result.NormalizedValue = normalized
	result.Valid = normalized != ""
	if !result.Valid {
		result.InvalidReasonCode = ReasonMissingExternalIdentifier
	}
	return result
}

func invalidIdentifier(value string, reason string, rules []string) NormalizedIdentifier {
	return NormalizedIdentifier{
		OriginalValue:        value,
		NormalizationVersion: NormalizationVersionV1,
		AppliedRules:         rules,
		Valid:                false,
		InvalidReasonCode:    reason,
	}
}

func hasControlRune(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r)
	}) >= 0
}

func collapseWhitespace(value string) string {
	var builder strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !lastSpace {
				builder.WriteRune(' ')
			}
			lastSpace = true
			continue
		}
		builder.WriteRune(r)
		lastSpace = false
	}
	return builder.String()
}

func normalizeDashForms(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '‐', '‑', '‒', '–', '—', '―', '－':
			return '-'
		default:
			return r
		}
	}, value)
}
