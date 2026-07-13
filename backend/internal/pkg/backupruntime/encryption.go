package backupruntime

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
)

const (
	encryptionMagic = "TMBK1"
	chunkSize       = 1024 * 1024
)

// Envelope records backup file encryption metadata safe for a manifest.
type Envelope struct {
	Algorithm      string `json:"algorithm"`
	KeyID          string `json:"keyId"`
	WrappedDataKey string `json:"wrappedDataKey"`
	ChunkSize      int    `json:"chunkSize"`
}

// EncryptFile streams src to dst using AES-GCM with a random data key. The data
// key is wrapped by the existing P4 encryption service; plaintext data keys are
// never written to disk.
func EncryptFile(src, dst, keyID string, kek *encrypt.Service) (*Envelope, error) {
	if kek == nil {
		return nil, fmt.Errorf("backup encryption: key encryption service unavailable")
	}
	dataKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dataKey); err != nil {
		return nil, fmt.Errorf("backup encryption: random key: %w", err)
	}
	wrapped, err := kek.Encrypt(dataKey)
	if err != nil {
		return nil, fmt.Errorf("backup encryption: wrap data key: %w", err)
	}
	if err := cryptFile(src, dst, dataKey, true); err != nil {
		return nil, err
	}
	return &Envelope{Algorithm: "AES-256-GCM-CHUNKED", KeyID: keyID, WrappedDataKey: wrapped, ChunkSize: chunkSize}, nil
}

// DecryptFile streams encrypted src to plaintext dst. Integrity failures abort
// before restore code can continue.
func DecryptFile(src, dst string, env Envelope, kek *encrypt.Service) error {
	if kek == nil {
		return fmt.Errorf("backup decrypt: key encryption service unavailable")
	}
	dataKey, err := kek.Decrypt(env.WrappedDataKey)
	if err != nil {
		return fmt.Errorf("backup decrypt: unwrap data key: %w", err)
	}
	if len(dataKey) != 32 {
		return fmt.Errorf("backup decrypt: invalid data key length")
	}
	return cryptFile(src, dst, dataKey, false)
}

func cryptFile(src, dst string, key []byte, encrypting bool) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if encrypting {
		if _, err := out.Write([]byte(encryptionMagic)); err != nil {
			return err
		}
		return encryptChunks(in, out, gcm)
	}
	magic := make([]byte, len(encryptionMagic))
	if _, err := io.ReadFull(in, magic); err != nil {
		return err
	}
	if string(magic) != encryptionMagic {
		return fmt.Errorf("backup decrypt: invalid magic")
	}
	return decryptChunks(in, out, gcm)
}

func encryptChunks(in io.Reader, out io.Writer, gcm cipher.AEAD) error {
	buf := make([]byte, chunkSize)
	var seq uint64
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			nonce := makeNonce(seq, gcm.NonceSize())
			ct := gcm.Seal(nil, nonce, buf[:n], nil)
			if err := binary.Write(out, binary.BigEndian, uint32(len(ct))); err != nil {
				return err
			}
			if _, err := out.Write(ct); err != nil {
				return err
			}
			seq++
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func decryptChunks(in io.Reader, out io.Writer, gcm cipher.AEAD) error {
	var seq uint64
	for {
		var l uint32
		if err := binary.Read(in, binary.BigEndian, &l); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if l == 0 || l > chunkSize+uint32(gcm.Overhead()) {
			return fmt.Errorf("backup decrypt: invalid chunk size")
		}
		ct := make([]byte, l)
		if _, err := io.ReadFull(in, ct); err != nil {
			return err
		}
		pt, err := gcm.Open(nil, makeNonce(seq, gcm.NonceSize()), ct, nil)
		if err != nil {
			return fmt.Errorf("backup decrypt: integrity check failed")
		}
		if _, err := out.Write(pt); err != nil {
			return err
		}
		seq++
	}
}

func makeNonce(seq uint64, size int) []byte {
	nonce := make([]byte, size)
	binary.BigEndian.PutUint64(nonce[size-8:], seq)
	return nonce
}
