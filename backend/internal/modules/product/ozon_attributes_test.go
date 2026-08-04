package product

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
)

func TestCanonicalOzonPlatformAttributesPreservesMultiValuesAndRepeatedComplexGroups(t *testing.T) {
	schema := []shop.PlatformCategoryAttribute{
		{AttrID: "10", Name: "Colors", Required: true, Options: datatypes.JSON([]byte(`[{"id":"1","value":"Red"},{"id":"2","value":"Blue"}]`)), Raw: datatypes.JSON([]byte(`{"dictionary_id":"7","is_collection":true,"max_value_count":2}`))},
		{AttrID: "20", Name: "Volume", Required: true, Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
		{AttrID: "21", Name: "Unit", Required: true, Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
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
		{AttrID: "10", Name: "Brand", Required: true, Raw: datatypes.JSON([]byte(`{"is_collection":false}`))},
		{AttrID: "20", Name: "Volume", Required: true, Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":false}`))},
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
		{AttrID: "20", Name: "Volume", Required: true, Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
		{AttrID: "21", Name: "Unit", Required: true, Raw: datatypes.JSON([]byte(`{"attribute_complex_id":9,"complex_is_collection":true}`))},
	}
	raw := datatypes.JSON([]byte(`{"version":2,"attributes":{},"complexGroups":[{"complexId":9,"attributes":{"20":[{"value":"100"}]}}]}`))
	if _, err := CanonicalOzonPlatformAttributes(schema, raw, false); err != nil {
		t.Fatalf("partial edit should be saveable: %v", err)
	}
	if _, err := CanonicalOzonPlatformAttributes(schema, raw, true); err == nil || !strings.Contains(err.Error(), "Unit") {
		t.Fatalf("preflight should block incomplete complex group: %v", err)
	}
}

func TestDecodeLegacyOzonAttributesWritesCanonicalV2(t *testing.T) {
	schema := []shop.PlatformCategoryAttribute{{AttrID: "85", Name: "Brand"}}
	canonical, err := CanonicalOzonPlatformAttributes(schema, datatypes.JSON([]byte(`{"85":{"value":"Acme"}}`)), false)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(canonical, &payload); err != nil || payload["version"] != float64(2) {
		t.Fatalf("legacy config was not canonicalized: %s err=%v", canonical, err)
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
