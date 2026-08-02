package passwordpolicy

import "testing"

func TestIsWeak(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		minimum  int
		weak     bool
	}{
		{name: "default rejects fewer than eight characters", password: "Safe123", weak: true},
		{name: "configured minimum is enforced", password: "SafePass", minimum: 12, weak: true},
		{name: "common weak password is rejected", password: "Qwerty123", weak: true},
		{name: "valid password passes", password: "SafePassphrase42!", minimum: 12, weak: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsWeak(tt.password, tt.minimum); got != tt.weak {
				t.Fatalf("IsWeak(%q, %d) = %v, want %v", tt.password, tt.minimum, got, tt.weak)
			}
		})
	}
}

func TestIsWeakWithForbidden(t *testing.T) {
	if !IsWeakWithForbidden("ProductionBootstrap42!", 8, "ProductionBootstrap42!") {
		t.Fatal("environment-specific forbidden password was accepted")
	}
	if IsWeakWithForbidden("ProductionBootstrap42!", 8) {
		t.Fatal("strong password was rejected without a matching forbidden value")
	}
}
