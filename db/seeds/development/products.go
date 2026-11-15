package development

import (
	"context"
	"database/sql"
	"errors"
	"go-todo/internal/store"
)

func seedProducts(ctx context.Context, queries *store.Queries, products []productSeed) error {
	for _, seed := range products {
		productID, err := queries.FindSeedProduct(ctx, seed.Name)
		if errors.Is(err, sql.ErrNoRows) {
			productID, err = queries.CreateProduct(ctx, store.CreateProductParams{
				Name: seed.Name, Description: seed.Description,
				Status: seed.Status, TaxRate: seed.TaxRate,
			})
		}
		if err != nil {
			return err
		}
		for _, sku := range seed.SKUs {
			if err := queries.CreateSeedSKU(ctx, store.CreateSeedSKUParams{
				ProductID: productID, Code: sku.Code,
				Size: sku.Size, Color: sku.Color, PriceYen: sku.PriceYen, StockQuantity: sku.StockQuantity,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
