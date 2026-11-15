package app

import (
	"errors"
	"net/mail"

	"go-todo/internal/domain"
	"go-todo/internal/store"
)

type Admin = domain.Admin
type Product = domain.Product
type SKU = domain.SKU

func adminModel(row store.Admin) Admin { return Admin{ID: row.ID, Email: row.Email, Role: row.Role} }
func productModel(row store.CatalogProduct) Product {
	return Product{ID: row.ID, Name: row.Name, Description: row.Description, Status: row.Status, TaxRate: row.TaxRate}
}
func skuModel(row store.CatalogSku) SKU {
	return SKU{ID: row.ID, ProductID: row.ProductID, Code: row.Code, Size: row.Size, Color: row.Color, PriceYen: row.PriceYen, StockQuantity: row.StockQuantity}
}

func validateEmail(email string) error {
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 {
		return errors.New("メールアドレスを正しく入力してください。")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 72 {
		return errors.New("パスワードは12〜72バイトで入力してください。")
	}
	return nil
}
