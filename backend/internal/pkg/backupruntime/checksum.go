package backupruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// SHA256File calculates a streaming SHA-256 checksum without loading the file.
func SHA256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// VerifySHA256File validates size and checksum.
func VerifySHA256File(path, expected string, minSize int64) error {
	sum, size, err := SHA256File(path)
	if err != nil {
		return err
	}
	if size <= minSize {
		return fmt.Errorf("backup checksum: file size %d is not above minimum %d", size, minSize)
	}
	if expected != "" && sum != expected {
		return fmt.Errorf("backup checksum mismatch")
	}
	return nil
}
