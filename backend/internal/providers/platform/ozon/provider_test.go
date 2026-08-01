package ozon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathSellerInfo {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Client-Id") != "123456" || r.Header.Get("Api-Key") != "secret-key" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":20,"message":"bad credentials"}`))
			return
		}
		_, _ = w.Write([]byte(`{"company":{"name":"Valore Elite","legal_name":"X Ltd","country":"CHN","currency":"CNY","inn":"123"},"subscription":{"is_premium":true,"type":"PREMIUM"}}`))
	}))
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	res, err := prov.TestConnection(context.Background(), platformp.TestConnectionRequest{
		AppKey:      "123456",
		AccessToken: "secret-key",
		Extra:       map[string]string{"api_base_url": srv.URL},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %+v", res)
	}
	if res.ShopName != "Valore Elite" || res.Currency != "CNY" || res.Region != "CHN" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestTestConnectionBadCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":20,"message":"invalid api key"}`))
	}))
	t.Cleanup(srv.Close)

	prov := ozonProvider{}
	res, err := prov.TestConnection(context.Background(), platformp.TestConnectionRequest{
		AppKey:      "1",
		AccessToken: "bad",
		Extra:       map[string]string{"api_base_url": srv.URL},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Message, "permission") && !strings.Contains(res.Message, "invalid api key") {
		t.Fatalf("unexpected message: %s", res.Message)
	}
}

func TestMetadata(t *testing.T) {
	prov := ozonProvider{}
	if prov.Platform() != "ozon" || prov.Name() != "Ozon" {
		t.Fatalf("unexpected metadata: %s %s", prov.Platform(), prov.Name())
	}
	if prov.Status() != platformp.StatusBeta {
		t.Fatalf("unexpected status: %s", prov.Status())
	}
	caps := prov.Capabilities()
	if len(caps) != 2 || !platformp.HasCapability(prov, platformp.CapProductPublish) {
		t.Fatalf("unexpected capabilities: %v", caps)
	}
	sch := prov.AuthSchema()
	if sch.AuthType != "api_key" || len(sch.Fields) != 2 {
		t.Fatalf("unexpected auth schema: %+v", sch)
	}
	if prov.AppConfigSchema().GroupKey != "" {
		t.Fatalf("ozon should not require deploy-level app settings")
	}
	if gk := prov.PublishConfigSchema().GroupKey; gk != "platform_publish_ozon" {
		t.Fatalf("unexpected publish group: %s", gk)
	}
}
