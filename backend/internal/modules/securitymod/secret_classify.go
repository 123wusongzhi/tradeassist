package securitymod

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
)

const legacyKeyID = "legacy"

// ciphertextStatus classifies encrypted values for rotation.
type ciphertextStatus int

const (
	ciphertextActive ciphertextStatus = iota
	ciphertextNeedsReencrypt
	ciphertextUnknown
	ciphertextEmpty
)

func classifyCiphertext(kr *crypto.KeyRing, v string, previousKeyIDs []string) (ciphertextStatus, string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return ciphertextEmpty, ""
	}
	if kid, ok := crypto.ParseKeyID(v); ok {
		if kr != nil && kid == kr.ActiveID {
			return ciphertextActive, kid
		}
		if len(previousKeyIDs) == 0 || containsKey(previousKeyIDs, kid) {
			return ciphertextNeedsReencrypt, kid
		}
		return ciphertextActive, kid
	}
	if kr != nil {
		if _, err := kr.Decrypt(v); err == nil {
			return ciphertextNeedsReencrypt, legacyKeyID
		}
	}
	return ciphertextUnknown, "unknown"
}
