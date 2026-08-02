package product

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestExternalCollectImageURLRejectsUnsafeTargets(t *testing.T) {
	originalResolver := resolveImageHost
	resolveImageHost = func(_ context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.1")}, nil
	}
	t.Cleanup(func() { resolveImageHost = originalResolver })

	for _, rawURL := range []string{
		"https://user:pass@img.alicdn.com/a.jpg",
		"http://127.0.0.1/a.jpg",
		"http://[::1]/a.jpg",
		"https://localhost/a.jpg",
		"https://img.alicdn.com.evil.example/a.jpg",
		"https://private.alicdn.com/a.jpg",
	} {
		if isExternalCollectImageURL(rawURL) {
			t.Fatalf("unsafe URL accepted: %s", rawURL)
		}
	}
}

func TestExternalCollectImageURLAcceptsPublicCDNAddress(t *testing.T) {
	originalResolver := resolveImageHost
	resolveImageHost = func(_ context.Context, host string) ([]net.IP, error) {
		if host != "img.alicdn.com" {
			t.Fatalf("unexpected host lookup: %s", host)
		}
		return []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("1.1.1.1")}, nil
	}
	t.Cleanup(func() { resolveImageHost = originalResolver })

	if !isExternalCollectImageURL("https://img.alicdn.com/image.jpg") {
		t.Fatal("public CDN URL rejected")
	}
}

func TestPublicImageIPRejectsReservedRanges(t *testing.T) {
	for _, rawIP := range []string{
		"0.0.0.1", "127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "192.0.2.1", "192.88.99.1", "198.18.0.1", "224.0.0.1", "240.0.0.1", "::1", "64:ff9b::1", "2001:db8::1", "2002::1", "fc00::1", "fe80::1",
	} {
		if isPublicImageIP(net.ParseIP(rawIP)) {
			t.Fatalf("reserved IP accepted: %s", rawIP)
		}
	}
	if !isPublicImageIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP rejected")
	}
}

func TestExternalCollectImageURLBoundsDNSLookup(t *testing.T) {
	originalResolver := resolveImageHost
	resolveImageHost = func(ctx context.Context, _ string) ([]net.IP, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() { resolveImageHost = originalResolver })

	started := time.Now()
	if isExternalCollectImageURL("https://img.alicdn.com/image.jpg") {
		t.Fatal("timed-out DNS lookup was accepted")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("DNS validation exceeded its bound: %s", elapsed)
	}
}
