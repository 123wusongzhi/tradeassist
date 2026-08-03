package ozon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func TestFetchCategoryTreeFlattensRecursiveOfficialShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathCategoryTree {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"description_category_id":100,"category_name":"Root","children":[{"description_category_id":110,"category_name":"Middle","disabled":true,"children":[{"type_id":200,"type_name":"Leaf","children":[]}]}]}]}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(platformp.TestConnectionRequest{AppKey: "client", AccessToken: "key", Extra: map[string]string{"api_base_url": srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := client.FetchCategoryTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("node count = %d, want 3: %#v", len(nodes), nodes)
	}
	if got := nodes[1]; got.DescriptionCategoryID != "110" || got.ParentID != "100" || got.Level != 2 || got.IsLeaf {
		t.Fatalf("middle node = %#v", got)
	}
	if got := nodes[2]; got.DescriptionCategoryID != "110" || got.TypeID != "200" || got.ParentID != "110" || got.Level != 3 || !got.IsLeaf || !got.Disabled {
		t.Fatalf("leaf node = %#v", got)
	}
}
