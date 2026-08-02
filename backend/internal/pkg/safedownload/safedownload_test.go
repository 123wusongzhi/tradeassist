package safedownload

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net"
	"strings"
	"testing"
)

type sequenceResolver struct {
	answers [][]net.IPAddr
	calls   int
}

func (r *sequenceResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	idx := r.calls
	r.calls++
	if idx >= len(r.answers) {
		return nil, errors.New("unexpected extra lookup")
	}
	return r.answers[idx], nil
}

func TestValidateURLBlocksLocalhost(t *testing.T) {
	for _, u := range []string{
		"http://localhost/x.png",
		"http://127.0.0.1/x.png",
		"http://[::1]/x.png",
		"http://10.0.0.1/x.png",
		"http://172.16.0.1/x.png",
		"http://192.168.1.1/x.png",
		"http://169.254.169.254/latest/meta-data",
	} {
		if err := ValidateURL(context.Background(), u); err == nil {
			t.Fatalf("expected block for %s", u)
		}
	}
}

func TestValidateURLBlocksCredentials(t *testing.T) {
	err := ValidateURL(context.Background(), "http://user:pass@example.com/x.png")
	if err == nil || !strings.Contains(err.Error(), ErrCredentialsInURL) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateURLBlocksNonHTTP(t *testing.T) {
	err := ValidateURL(context.Background(), "file:///etc/passwd")
	if err == nil {
		t.Fatal("expected scheme block")
	}
}

func TestIsPrivateIP(t *testing.T) {
	if !IsPrivateIP(parseIP("10.1.2.3")) {
		t.Fatal("10.x should be private")
	}
	for _, raw := range []string{"100.64.0.1", "192.0.2.1", "198.18.0.1", "203.0.113.1", "2001:db8::1", "2002::1"} {
		if !IsPrivateIP(parseIP(raw)) {
			t.Fatalf("reserved address %s should be blocked", raw)
		}
	}
}

func TestDialPublicContextPinsValidatedAddress(t *testing.T) {
	resolver := &sequenceResolver{answers: [][]net.IPAddr{
		{{IP: net.ParseIP("8.8.8.8")}},
		{{IP: net.ParseIP("127.0.0.1")}},
	}}
	var dialed string
	sentinel := errors.New("stop after capture")
	_, err := dialPublicContext(t.Context(), "tcp", "images.example:443", resolver, func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = addr
		return nil, sentinel
	})
	if err == nil || !strings.Contains(err.Error(), sentinel.Error()) {
		t.Fatalf("expected captured dial error, got %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected one DNS lookup, got %d", resolver.calls)
	}
	if dialed != "8.8.8.8:443" {
		t.Fatalf("expected exact validated IP, got %q", dialed)
	}
}

func TestDialPublicContextRejectsMixedPublicPrivateAnswers(t *testing.T) {
	resolver := &sequenceResolver{answers: [][]net.IPAddr{{
		{IP: net.ParseIP("8.8.8.8")},
		{IP: net.ParseIP("127.0.0.1")},
	}}}
	called := false
	_, err := dialPublicContext(t.Context(), "tcp", "images.example:443", resolver, func(_ context.Context, _, _ string) (net.Conn, error) {
		called = true
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), ErrPrivateIP) {
		t.Fatalf("expected private address rejection, got %v", err)
	}
	if called {
		t.Fatal("dial must not run for mixed public/private DNS answers")
	}
}

func TestValidateImageBytesRejectsExcessiveDimensions(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 9000, 1))); err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	if err := ValidateImageBytes(buf.Bytes(), "image/png", opts); err == nil || !strings.Contains(err.Error(), ErrImageDimensions) {
		t.Fatalf("expected dimension rejection, got %v", err)
	}
}

func TestValidateImageBytesRejectsExcessivePixels(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 100, 100))); err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.MaxImagePixels = 5_000
	if err := ValidateImageBytes(buf.Bytes(), "image/png", opts); err == nil || !strings.Contains(err.Error(), ErrImageDimensions) {
		t.Fatalf("expected pixel rejection, got %v", err)
	}
}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
