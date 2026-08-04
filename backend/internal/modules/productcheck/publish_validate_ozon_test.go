package productcheck

import "testing"

func TestOzonPublishOptionStringTreatsMissingAndNilAsEmpty(t *testing.T) {
	for name, options := range map[string]map[string]any{
		"missing": {},
		"nil":     {"currency_code": nil},
		"wrong":   {"currency_code": 123},
	} {
		t.Run(name, func(t *testing.T) {
			if got := ozonPublishOptionString(options, "currency_code"); got != "" {
				t.Fatalf("ozonPublishOptionString() = %q, want empty", got)
			}
		})
	}
	if got := ozonPublishOptionString(map[string]any{"currency_code": " RUB "}, "currency_code"); got != "RUB" {
		t.Fatalf("ozonPublishOptionString() = %q, want RUB", got)
	}
}

func TestOzonPackageDimensionsRequirePositiveValues(t *testing.T) {
	keys := supplementalPublishRequiredKeys("ozon")
	if len(keys) != 4 {
		t.Fatalf("Ozon required package keys = %#v", keys)
	}
	for _, raw := range []string{"", "0", "-1", "abc"} {
		if positiveOzonPackageValue(raw) {
			t.Fatalf("invalid Ozon package value accepted: %q", raw)
		}
	}
	if !positiveOzonPackageValue("0.1") {
		t.Fatal("positive Ozon package value rejected")
	}
}
