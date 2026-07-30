package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const encPrefix = "enc:v2:"

// KeyRing supports versioned APP_MASTER_KEY encryption (enc:v2:{keyId}:{nonce}:{ciphertext}).
type KeyRing struct {
	ActiveID     string
	ActiveKey    []byte
	PreviousKeys map[string][]byte
}

// NewKeyRing builds a key ring from active and optional previous keys JSON.
// previousJSON format: {"keyId1":"hexOrBase64Key","keyId2":"..."}
func NewKeyRing(activeID, activeKey string, previousJSON string) (*KeyRing, error) {
	kr := &KeyRing{PreviousKeys: map[string][]byte{}}
	activeID = strings.TrimSpace(activeID)
	if activeID == "" {
		activeID = "default"
	}
	key, err := parseKeyMaterial(activeKey)
	if err != nil {
		return nil, err
	}
	kr.ActiveID = activeID
	kr.ActiveKey = key
	prev := strings.TrimSpace(previousJSON)
	if prev != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(prev), &m); err != nil {
			return nil, fmt.Errorf("keyring: invalid previous keys json: %w", err)
		}
		for id, mat := range m {
			b, err := parseKeyMaterial(mat)
			if err != nil {
				return nil, fmt.Errorf("keyring: previous key %q: %w", id, err)
			}
			kr.PreviousKeys[strings.TrimSpace(id)] = b
		}
	}
	return kr, nil
}

// Encrypt returns versioned ciphertext string.
func (kr *KeyRing) Encrypt(plaintext []byte) (string, error) {
	if kr == nil || len(kr.ActiveKey) == 0 {
		return "", fmt.Errorf("keyring: no active key")
	}
	if len(plaintext) == 0 {
		return "", nil
	}
	block, err := aes.NewCipher(kr.ActiveKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return fmt.Sprintf("%s%s:%s:%s", encPrefix, kr.ActiveID, base64.RawURLEncoding.EncodeToString(nonce), base64.RawURLEncoding.EncodeToString(ct)), nil
}

// Decrypt decodes versioned or legacy base64 ciphertext.
func (kr *KeyRing) Decrypt(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, nil
	}
	if strings.HasPrefix(encoded, encPrefix) {
		return kr.decryptVersioned(encoded)
	}
	// Legacy encrypt.Service format: base64(nonce||ciphertext) with active key only.
	return kr.decryptLegacy(encoded, kr.ActiveKey)
}

func (kr *KeyRing) decryptVersioned(encoded string) ([]byte, error) {
	body := strings.TrimPrefix(encoded, encPrefix)
	parts := strings.SplitN(body, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("keyring: invalid ciphertext format")
	}
	keyID, nonceB64, ctB64 := parts[0], parts[1], parts[2]
	key, ok := kr.keyByID(keyID)
	if !ok {
		return nil, fmt.Errorf("keyring: unknown key id %q", keyID)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, err
	}
	ct, err := base64.RawURLEncoding.DecodeString(ctB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, nil)
}

func (kr *KeyRing) keyByID(id string) ([]byte, bool) {
	if kr == nil {
		return nil, false
	}
	if id == kr.ActiveID {
		return kr.ActiveKey, len(kr.ActiveKey) > 0
	}
	b, ok := kr.PreviousKeys[id]
	return b, ok && len(b) > 0
}

func (kr *KeyRing) decryptLegacy(encoded string, key []byte) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("keyring: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// IsEncrypted reports whether value uses versioned prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), encPrefix)
}

// ParseKeyID extracts key id from enc:v2 ciphertext.
func ParseKeyID(encoded string) (string, bool) {
	encoded = strings.TrimSpace(encoded)
	if !strings.HasPrefix(encoded, encPrefix) {
		return "", false
	}
	body := strings.TrimPrefix(encoded, encPrefix)
	parts := strings.SplitN(body, ":", 3)
	if len(parts) < 1 || parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

// PreviousKeyIDs returns configured previous key ids.
func (kr *KeyRing) PreviousKeyIDs() []string {
	if kr == nil || len(kr.PreviousKeys) == 0 {
		return nil
	}
	out := make([]string, 0, len(kr.PreviousKeys))
	for id := range kr.PreviousKeys {
		out = append(out, id)
	}
	return out
}

func parseKeyMaterial(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty key material")
	}
	if len(s) == 64 {
		if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(s) == 32 {
		return []byte(s), nil
	}
	sum := sha256.Sum256([]byte(s))
	return sum[:], nil
}
