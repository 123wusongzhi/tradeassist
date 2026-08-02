package files

import "testing"

func TestRequirePrivateQuarantineStorageFailsClosedForRemoteProviders(t *testing.T) {
	if err := requirePrivateQuarantineStorage("local", nil); err != nil {
		t.Fatalf("local quarantine should use the guarded /static path: %v", err)
	}
	if err := requirePrivateQuarantineStorage("local", map[string]string{"public_base": "/static/"}); err != nil {
		t.Fatalf("explicit guarded /static path should be accepted: %v", err)
	}
	for _, publicBase := range []string{"/uploads", "https://cdn.example.test/static"} {
		if err := requirePrivateQuarantineStorage("local", map[string]string{"public_base": publicBase}); err == nil {
			t.Fatalf("direct local public_base %q should fail closed", publicBase)
		}
	}
	for _, kind := range []string{"s3", "r2", "minio", "cos", "oss", "unknown"} {
		if err := requirePrivateQuarantineStorage(kind, nil); err == nil {
			t.Fatalf("%s quarantine should fail closed without a private provider", kind)
		}
	}
}
