package productpublish

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateOzonCategoryGroupProductsRejectsOverlap(t *testing.T) {
	productID := uuid.New().String()
	svc := &Service{BatchMaxProducts: 10}
	_, err := svc.validateOzonCategoryGroupProducts([]OzonCategoryGroupConfirm{
		{ProductIDs: []string{productID}},
		{ProductIDs: []string{productID}},
	})
	if err == nil || !strings.Contains(err.Error(), "more than one category group") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
}

func TestValidateOzonCategoryGroupProductsEnforcesTotalLimit(t *testing.T) {
	svc := &Service{BatchMaxProducts: 2}
	_, err := svc.validateOzonCategoryGroupProducts([]OzonCategoryGroupConfirm{
		{ProductIDs: []string{uuid.New().String(), uuid.New().String()}},
		{ProductIDs: []string{uuid.New().String()}},
	})
	if err == nil {
		t.Fatal("expected total product limit rejection")
	}
}
