package operationtask

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"sort"
	"strings"
)

const CanonicalJSONHashVersion = 1

func ComputePayloadHash(raw []byte) (string, error) {
	canonical, err := CanonicalizeJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func CanonicalizeJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, ErrValidation
	}
	var extra any
	if err := dec.Decode(&extra); !errorsIsEOF(err) {
		return nil, ErrValidation
	}
	var b bytes.Buffer
	if err := writeCanonicalJSON(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func errorsIsEOF(err error) bool {
	return err == io.EOF
}

func writeCanonicalJSON(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		n, err := canonicalNumber(x.String())
		if err != nil {
			return err
		}
		b.WriteString(n)
	case string:
		encoded, err := json.Marshal(x)
		if err != nil {
			return ErrValidation
		}
		b.Write(encoded)
	case []any:
		b.WriteByte('[')
		for i, child := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeCanonicalJSON(b, child); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			encodedKey, err := json.Marshal(k)
			if err != nil {
				return ErrValidation
			}
			b.Write(encodedKey)
			b.WriteByte(':')
			if err := writeCanonicalJSON(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return ErrValidation
	}
	return nil
}

func canonicalNumber(in string) (string, error) {
	if in == "" {
		return "", ErrValidation
	}
	sign := ""
	if in[0] == '-' {
		sign = "-"
		in = in[1:]
	}
	mantissa := in
	exp := 0
	if idx := strings.IndexAny(in, "eE"); idx >= 0 {
		mantissa = in[:idx]
		expPart := in[idx+1:]
		parsedExp, ok := new(big.Int).SetString(strings.TrimPrefix(expPart, "+"), 10)
		if !ok || !parsedExp.IsInt64() {
			return "", ErrValidation
		}
		exp = int(parsedExp.Int64())
	}
	scale := 0
	if idx := strings.IndexByte(mantissa, '.'); idx >= 0 {
		scale = len(mantissa) - idx - 1
		mantissa = mantissa[:idx] + mantissa[idx+1:]
	}
	if mantissa == "" {
		return "", ErrValidation
	}
	for _, ch := range mantissa {
		if ch < '0' || ch > '9' {
			return "", ErrValidation
		}
	}
	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return "0", nil
	}
	totalScale := scale - exp
	if totalScale <= 0 {
		return sign + digits + strings.Repeat("0", -totalScale), nil
	}
	if totalScale >= len(digits) {
		out := sign + "0." + strings.Repeat("0", totalScale-len(digits)) + digits
		return strings.TrimRight(out, "0"), nil
	}
	cut := len(digits) - totalScale
	out := sign + digits[:cut] + "." + digits[cut:]
	out = strings.TrimRight(out, "0")
	return strings.TrimSuffix(out, "."), nil
}
