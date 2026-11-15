package development

import (
	"fmt"
	"net/mail"
	"strings"

	"go-todo/internal/domain"
)

func validateData(data *seedData) error {
	if len(data.Admins.Admins) > 0 && (len(data.Admins.Password) == 0 || len(data.Admins.Password) > 72) {
		return fmt.Errorf("admins.yml: password は1〜72バイトで指定してください")
	}
	emails := make(map[string]bool)
	for index := range data.Admins.Admins {
		admin := &data.Admins.Admins[index]
		admin.Email = strings.ToLower(strings.TrimSpace(admin.Email))
		address, err := mail.ParseAddress(admin.Email)
		if err != nil || address.Address != admin.Email || len(admin.Email) > 254 {
			return fmt.Errorf("admins.yml: admins[%d] のメールアドレスが不正です", index)
		}
		if admin.Role != "super_admin" && admin.Role != "operator" {
			return fmt.Errorf("admins.yml: %s の role が不正です", admin.Email)
		}
		if emails[admin.Email] {
			return fmt.Errorf("admins.yml: メールアドレス %s が重複しています", admin.Email)
		}
		emails[admin.Email] = true
	}
	names := make(map[string]bool)
	codes := make(map[string]bool)
	for index := range data.Products {
		product := &data.Products[index]
		product.Name = strings.TrimSpace(product.Name)
		if err := (domain.Product{Name: product.Name, Description: product.Description, Status: product.Status, TaxRate: product.TaxRate}).Validate(); err != nil {
			return fmt.Errorf("products.yml: products[%d]: %w", index, err)
		}
		if names[product.Name] {
			return fmt.Errorf("products.yml: 商品名 %s が重複しています", product.Name)
		}
		names[product.Name] = true
		variants := make(map[[2]string]bool)
		for skuIndex := range product.SKUs {
			sku := &product.SKUs[skuIndex]
			model := domain.SKU{Code: sku.Code, Size: sku.Size, Color: sku.Color, PriceYen: sku.PriceYen, StockQuantity: sku.StockQuantity}
			model.Normalize()
			if err := model.Validate(); err != nil {
				return fmt.Errorf("products.yml: %s skus[%d]: %w", product.Name, skuIndex, err)
			}
			sku.Code, sku.Size, sku.Color = model.Code, model.Size, model.Color
			if codes[sku.Code] {
				return fmt.Errorf("products.yml: SKUコード %s が重複しています", sku.Code)
			}
			codes[sku.Code] = true
			variant := [2]string{sku.Size, sku.Color}
			if variants[variant] {
				return fmt.Errorf("products.yml: %s のサイズ・カラーが重複しています", product.Name)
			}
			variants[variant] = true
		}
	}
	return nil
}
