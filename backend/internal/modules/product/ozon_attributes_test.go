package product

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
)

func TestCanonicalOzonPlatformAttributesPreservesMultiValuesAndRepeatedComplexGroups(t *testing.T) {
	schema := []shop.PlatformCategoryAttribute{
		{AttrID: "10", Name: "Colors", Required: true, ValueType: "String", Options: datatypes.JSON([]byte(`[{"id":"1","value":"Red"},{"id":"2","value":"Blue"}]`)), Raw: datatypes.JSON([]byte(`{"dictionary_id":"7","is_collection":true,"max_value_count":2}`))},
		{AttrID: "20", Name: "Volume", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
		{AttrID: "21", Name: "Unit", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
	}
	raw := datatypes.JSON([]byte(`{
		"version":2,
		"attributes":{"10":[{"value":"Red","dictionaryValueId":"1"},{"value":"Blue","dictionaryValueId":"2"}]},
		"complexGroups":[
			{"complexId":9,"attributes":{"20":[{"value":"100"}],"21":[{"value":"ml"}]}},
			{"complexId":9,"attributes":{"20":[{"value":"200"}],"21":[{"value":"ml"}]}}
		]
	}`))
	canonical, err := CanonicalOzonPlatformAttributes(schema, raw, true)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := DecodeOzonPlatformAttributes(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Attributes["10"]) != 2 || len(payload.ComplexGroups) != 2 || payload.ComplexGroups[1].Attributes["20"][0].Value != "200" {
		t.Fatalf("canonical payload lost Ozon collection semantics: %+v", payload)
	}
}

func TestOzonPlatformAttributesRejectsSilentSingleValueCoercion(t *testing.T) {
	schema := []shop.PlatformCategoryAttribute{
		{AttrID: "10", Name: "Brand", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"is_collection":false}`))},
		{AttrID: "20", Name: "Volume", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":false}`))},
	}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "ordinary multi on scalar", raw: `{"version":2,"attributes":{"10":[{"value":"A"},{"value":"B"}]},"complexGroups":[]}`, want: "不是多值属性"},
		{name: "complex stored as ordinary", raw: `{"version":2,"attributes":{"20":[{"value":"100"}]},"complexGroups":[]}`, want: "可重复字段组"},
		{name: "repeated non collection complex", raw: `{"version":2,"attributes":{"10":[{"value":"A"}]},"complexGroups":[{"complexId":9,"attributes":{"20":[{"value":"1"}]}},{"complexId":9,"attributes":{"20":[{"value":"2"}]}}]}`, want: "不允许重复"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CanonicalOzonPlatformAttributes(schema, datatypes.JSON([]byte(tt.raw)), true)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
		})
	}
}

func TestOzonPlatformAttributesAllowsIncompleteSaveButBlocksIncompletePreflight(t *testing.T) {
	schema := []shop.PlatformCategoryAttribute{
		{AttrID: "20", Name: "Volume", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
		{AttrID: "21", Name: "Unit", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
	}
	raw := datatypes.JSON([]byte(`{"version":2,"attributes":{},"complexGroups":[{"complexId":9,"attributes":{"20":[{"value":"100"}]}}]}`))
	if _, err := CanonicalOzonPlatformAttributes(schema, raw, false); err != nil {
		t.Fatalf("partial edit should be saveable: %v", err)
	}
	if _, err := CanonicalOzonPlatformAttributes(schema, raw, true); err == nil || !strings.Contains(err.Error(), "Unit") {
		t.Fatalf("preflight should block incomplete complex group: %v", err)
	}
}

func TestDecodeLegacyOzonAttributesWritesCanonicalV3(t *testing.T) {
	schema := []shop.PlatformCategoryAttribute{{AttrID: "85", Name: "Brand", ValueType: "String"}}
	canonical, err := CanonicalOzonPlatformAttributes(schema, datatypes.JSON([]byte(`{"85":{"value":"Acme"}}`)), false)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(canonical, &payload); err != nil || payload["version"] != float64(3) {
		t.Fatalf("legacy config was not canonicalized: %s err=%v", canonical, err)
	}
}

func TestOzonSKUVariantAttributesRequireCompleteUniqueMappings(t *testing.T) {
	redID := uuid.New()
	blueID := uuid.New()
	schema := []shop.PlatformCategoryAttribute{
		{AttrID: "85", Name: "Brand", Required: true, ValueType: "String", Raw: datatypes.JSON([]byte(`{"is_aspect":false,"is_collection":false}`))},
		{AttrID: "10096", Name: "Color", Required: true, ValueType: "String", Options: datatypes.JSON([]byte(`[{"id":"1","value":"Red"},{"id":"2","value":"Blue"}]`)), Raw: datatypes.JSON([]byte(`{"dictionary_id":"7","is_aspect":true,"is_collection":false}`))},
	}
	valid := datatypes.JSON([]byte(fmt.Sprintf(`{
		"version":3,
		"attributes":{"85":[{"value":"Acme"}]},
		"complexGroups":[],
		"skuVariantAttributeIds":["10096"],
		"skuAttributeOverrides":{
			%q:{"10096":[{"value":"Red","dictionaryValueId":"1"}]},
			%q:{"10096":[{"value":"Blue","dictionaryValueId":"2"}]}
		}
	}`, redID.String(), blueID.String())))
	canonical, err := CanonicalOzonPlatformAttributesForSKUs(schema, valid, []uuid.UUID{redID, blueID}, true)
	if err != nil {
		t.Fatalf("valid SKU mapping rejected: %v", err)
	}
	payload, err := DecodeOzonPlatformAttributes(canonical)
	if err != nil {
		t.Fatal(err)
	}
	red := ResolveOzonEffectiveSKUAttributes(payload, redID.String())
	blue := ResolveOzonEffectiveSKUAttributes(payload, blueID.String())
	if red.Attributes["10096"][0].Value != "Red" || blue.Attributes["10096"][0].Value != "Blue" || red.Attributes["85"][0].Value != "Acme" {
		t.Fatalf("per-SKU attributes were not resolved: red=%+v blue=%+v", red, blue)
	}
	if _, err := CanonicalOzonPlatformAttributesForSKUs(schema, valid, []uuid.UUID{}, false); err == nil || !strings.Contains(err.Error(), "不存在的 SKU") || !IsOzonSKUVariantValidationError(err) {
		t.Fatalf("SKU overrides must be rejected as a variant error when the product currently has no SKUs: %v", err)
	}
	uppercaseSKU := datatypes.JSON([]byte(strings.Replace(string(valid), redID.String(), strings.ToUpper(redID.String()), 1)))
	uppercaseCanonical, err := CanonicalOzonPlatformAttributesForSKUs(schema, uppercaseSKU, []uuid.UUID{redID, blueID}, true)
	if err != nil {
		t.Fatalf("valid UUID casing should be canonicalized: %v", err)
	}
	uppercasePayload, err := DecodeOzonPlatformAttributes(uppercaseCanonical)
	if err != nil || len(uppercasePayload.SKUAttributeOverrides[redID.String()]) == 0 {
		t.Fatalf("SKU UUID key was not canonicalized: payload=%+v err=%v", uppercasePayload, err)
	}

	duplicate := datatypes.JSON([]byte(strings.Replace(string(valid), `"value":"Blue","dictionaryValueId":"2"`, `"value":"Red","dictionaryValueId":"1"`, 1)))
	if _, err := CanonicalOzonPlatformAttributesForSKUs(schema, duplicate, []uuid.UUID{redID, blueID}, true); err == nil || !strings.Contains(err.Error(), "组合重复") || !IsOzonSKUVariantValidationError(err) {
		t.Fatalf("duplicate variant tuple should be blocked: %v", err)
	}

	missing := datatypes.JSON([]byte(strings.Replace(string(valid), fmt.Sprintf(`%q:{"10096":[{"value":"Blue","dictionaryValueId":"2"}]}`, blueID.String()), fmt.Sprintf(`%q:{}`, blueID.String()), 1)))
	if _, err := CanonicalOzonPlatformAttributesForSKUs(schema, missing, []uuid.UUID{redID, blueID}, false); err != nil {
		t.Fatalf("incomplete edit should remain saveable: %v", err)
	}
	if _, err := CanonicalOzonPlatformAttributesForSKUs(schema, missing, []uuid.UUID{redID, blueID}, true); err == nil || !strings.Contains(err.Error(), "缺少变体属性") || !IsOzonSKUVariantValidationError(err) {
		t.Fatalf("incomplete SKU mapping should block preflight: %v", err)
	}

	v2 := datatypes.JSON([]byte(`{"version":2,"attributes":{"85":[{"value":"Acme"}],"10096":[{"value":"Red","dictionaryValueId":"1"}]},"complexGroups":[]}`))
	if _, err := CanonicalOzonPlatformAttributesForSKUs(schema, v2, []uuid.UUID{redID, blueID}, true); err == nil || !strings.Contains(err.Error(), "必须选择至少一个") || !IsOzonSKUVariantValidationError(err) {
		t.Fatalf("legacy multi-SKU config should not silently reuse one value: %v", err)
	}

	productMissing := datatypes.JSON([]byte(`{"version":3,"attributes":{"10096":[{"value":"Red","dictionaryValueId":"1"}]},"complexGroups":[],"skuVariantAttributeIds":[],"skuAttributeOverrides":{}}`))
	if _, err := CanonicalOzonPlatformAttributesForSKUs(schema, productMissing, []uuid.UUID{redID}, true); err == nil || !strings.Contains(err.Error(), "Brand") || IsOzonSKUVariantValidationError(err) {
		t.Fatalf("shared required attributes must remain product-level validation errors: %v", err)
	}
}

func TestOzonSKUVariantEligibilityFailsClosed(t *testing.T) {
	firstID := uuid.New()
	secondID := uuid.New()
	payload := OzonPlatformAttributePayload{
		Version:                OzonPlatformAttributesVersion,
		Attributes:             map[string][]OzonAttributeSelection{},
		ComplexGroups:          []OzonComplexAttributeGroup{},
		SKUVariantAttributeIDs: []string{"85"},
		SKUAttributeOverrides: map[string]map[string][]OzonAttributeSelection{
			firstID.String():  {"85": {{Value: "Brand A"}}},
			secondID.String(): {"85": {{Value: "Brand B"}}},
		},
	}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "explicitly ineligible brand", raw: `{"is_aspect":false}`, want: "不允许作为 SKU 变体维度"},
		{name: "legacy template without evidence", raw: `{}`, want: "缺少 is_aspect 变体资格证据"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := []shop.PlatformCategoryAttribute{{AttrID: "85", Name: "Brand", ValueType: "String", Raw: datatypes.JSON([]byte(tt.raw))}}
			err := ValidateOzonPlatformAttributePayloadForSKUs(schema, payload, []uuid.UUID{firstID, secondID}, true)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !IsOzonSKUVariantValidationError(err) {
				t.Fatalf("error=%v want variant error containing %q", err, tt.want)
			}
		})
	}
}

func TestOzonAttributeValueTypeValidation(t *testing.T) {
	tests := []struct {
		name      string
		valueType string
		value     string
		wantError string
	}{
		{name: "integer", valueType: "Integer", value: "12", wantError: ""},
		{name: "integer overflow", valueType: "Integer", value: "9223372036854775808", wantError: "64 位整数"},
		{name: "decimal", valueType: "Decimal", value: "12.50", wantError: ""},
		{name: "decimal exponent", valueType: "Decimal", value: "1e3", wantError: "不支持科学计数法"},
		{name: "boolean", valueType: "Boolean", value: "true", wantError: ""},
		{name: "boolean invalid", valueType: "Boolean", value: "yes", wantError: "true 或 false"},
		{name: "url", valueType: "URL", value: "https://example.com/item", wantError: ""},
		{name: "url invalid scheme", valueType: "URL", value: "ftp://example.com/item", wantError: "http/https URL"},
		{name: "date", valueType: "Date", value: "2026-08-10", wantError: ""},
		{name: "unsupported", valueType: "GeoPoint", value: "1,2", wantError: "尚未受支持"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := []shop.PlatformCategoryAttribute{{AttrID: "10", Name: "Typed", ValueType: tt.valueType, Raw: datatypes.JSON([]byte(`{"is_collection":false}`))}}
			payload := OzonPlatformAttributePayload{Version: OzonPlatformAttributesVersion, Attributes: map[string][]OzonAttributeSelection{"10": {{Value: tt.value}}}}
			err := ValidateOzonPlatformAttributePayload(schema, payload, false)
			if tt.wantError == "" && err != nil {
				t.Fatalf("valid value rejected: %v", err)
			}
			if tt.wantError != "" && (err == nil || !strings.Contains(err.Error(), tt.wantError)) {
				t.Fatalf("error=%v want %q", err, tt.wantError)
			}
		})
	}
}

func TestOzonSKUCountGuardrailAppliesAtPreflightOnly(t *testing.T) {
	skuIDs := make([]uuid.UUID, shop.OzonMaxSKUsPerPublish+1)
	for i := range skuIDs {
		skuIDs[i] = uuid.New()
	}
	payload := OzonPlatformAttributePayload{Version: OzonPlatformAttributesVersion}
	if err := ValidateOzonPlatformAttributePayloadForSKUs(nil, payload, skuIDs, false); err != nil {
		t.Fatalf("draft save should remain incomplete but saveable: %v", err)
	}
	err := ValidateOzonPlatformAttributePayloadForSKUs(nil, payload, skuIDs, true)
	if err == nil || !strings.Contains(err.Error(), "SKU") || !strings.Contains(err.Error(), fmt.Sprint(shop.OzonMaxSKUsPerPublish)) {
		t.Fatalf("preflight must enforce SKU guardrail: %v", err)
	}
}

func TestDecodeOzonAttributesRecognizesComplexOnlyV2Payload(t *testing.T) {
	payload, err := DecodeOzonPlatformAttributes([]byte(`{"version":2,"complexGroups":[{"complexId":9,"attributes":{"20":[{"value":"100"}]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Attributes) != 0 || len(payload.ComplexGroups) != 1 || payload.ComplexGroups[0].ComplexID != 9 {
		t.Fatalf("complex-only v2 payload was not preserved: %+v", payload)
	}
}

func TestDecodeOzonAttributesPreservesJSONSyntaxContext(t *testing.T) {
	_, err := DecodeOzonPlatformAttributes([]byte(`{"version":2,`))
	if err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("syntax context missing from error: %v", err)
	}
}
