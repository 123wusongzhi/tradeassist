package shop

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
)

// OzonWarehouseDTO is an operator-facing warehouse option. It deliberately
// contains no credentials and is fetched for one tenant-scoped shop.
type OzonWarehouseDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsRFBS  bool   `json:"isRfbs"`
	IsKGT   bool   `json:"isKgt"`
	Economy bool   `json:"economy"`
}

func (s *Service) ListOzonWarehouses(ctx context.Context, tenantID int64, shopID uuid.UUID) ([]OzonWarehouseDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	ctx = security.WithTenantContext(ctx, &security.TenantContext{TenantID: tenantID})
	_, auth, err := s.resolveOzonShopAndAuth(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	client, err := platformozon.NewClient(auth)
	if err != nil {
		return nil, ozonCategoryErr(OzonShopRequired, err)
	}
	rows, err := client.FetchWarehouses(ctx)
	if err != nil {
		return nil, ozonCategoryErr("ozon_warehouse_list_failed", err)
	}
	out := make([]OzonWarehouseDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, OzonWarehouseDTO{
			ID: row.ID, Name: row.Name, IsRFBS: row.IsRFBS, IsKGT: row.IsKGT, Economy: row.Economy,
		})
	}
	return out, nil
}
