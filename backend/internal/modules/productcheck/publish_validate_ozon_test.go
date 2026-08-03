package productcheck

import "testing"

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
