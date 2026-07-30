package p7diag

import (
	"strings"
	"time"
)

const (
	MetricPasswordVerifyDuration = "p7_diag_password_verify_duration_ms"
	PasswordAlgoBcrypt           = "bcrypt"
	PathUnknownAccount           = "unknown_account"
	PathKnownWrongPassword       = "known_account_wrong_password"
	PathLockedAccount            = "locked_account"
	PathSuccessVerify            = "known_account_verify_success"
)

// PasswordVerifyFields are safe password-verify diagnostics (no password/hash).
type PasswordVerifyFields struct {
	Algorithm             string `json:"algorithm,omitempty"`
	ConfiguredCost        int    `json:"configuredCost,omitempty"`
	VerifyCountPerRequest int    `json:"verifyCountPerRequest,omitempty"`
	Path                  string `json:"path,omitempty"`
}

// ObservePasswordVerify records CPU-bound password verification duration for a fixed path.
func ObservePasswordVerify(path, algorithm string, configuredCost, verifyCountPerRequest int, started time.Time) {
	if !Enabled() || started.IsZero() || !validPasswordPath(path) {
		return
	}
	algorithm = strings.TrimSpace(strings.ToLower(algorithm))
	if algorithm == "" {
		algorithm = PasswordAlgoBcrypt
	}
	if configuredCost < 0 {
		configuredCost = 0
	}
	if verifyCountPerRequest < 0 {
		verifyCountPerRequest = 0
	}
	outcome := OutcomeExpectedRejection
	if path == PathSuccessVerify {
		outcome = OutcomeSuccess
	}
	emit(Event{
		Metric:     MetricPasswordVerifyDuration,
		Type:       "password_verify",
		Route:      RouteAuthInvalidLogin,
		Stage:      "password_verify",
		Outcome:    outcome,
		PathType:   path,
		DurationMs: durationMs(time.Since(started)),
		Password: &PasswordVerifyFields{
			Algorithm:             algorithm,
			ConfiguredCost:        configuredCost,
			VerifyCountPerRequest: verifyCountPerRequest,
			Path:                  path,
		},
	})
}

func validPasswordPath(v string) bool {
	switch v {
	case PathUnknownAccount, PathKnownWrongPassword, PathLockedAccount, PathSuccessVerify:
		return true
	default:
		return false
	}
}
