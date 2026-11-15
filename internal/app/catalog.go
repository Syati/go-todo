package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-todo/internal/store"
	"go-todo/internal/views/components"
	productviews "go-todo/internal/views/products"
	skuviews "go-todo/internal/views/skus"
)

func pagination(r *http.Request) (int, int) {
	n, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if n < 1 || n > 1000000 {
		n = 1
	}
	return n, (n - 1) * 30
}

func (a *App) products(w http.ResponseWriter, r *http.Request) {
	n, offset := pagination(r)
	p := productviews.IndexData{Pagination: components.Pagination{Query: r.URL.Query().Get("q"), Status: r.URL.Query().Get("status"), Sort: r.URL.Query().Get("sort"), Page: n}}
	rows, err := a.queries.ListProducts(r.Context(), store.ListProductsParams{Search: p.Query, StatusFilter: p.Status, Ascending: p.Sort == "asc", PageOffset: offset})
	if err != nil {
		a.fail(w, err)
		return
	}
	for _, row := range rows {
		p.Products = append(p.Products, productModel(row))
	}
	if len(p.Products) > 30 {
		p.Products = p.Products[:30]
		p.Next = n + 1
	}
	if n > 1 {
		p.Previous = n - 1
	}
	a.render(w, r, productviews.Index(a.viewData(w, r, "商品一覧"), p), 200)
}

func (a *App) findProduct(w http.ResponseWriter, r *http.Request) (Product, bool) {
	n, err := id(r)
	if err != nil {
		http.NotFound(w, r)
		return Product{}, false
	}
	row, err := a.queries.FindProduct(r.Context(), n)
	p := productModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return p, false
	}
	if err != nil {
		a.fail(w, err)
		return p, false
	}
	return p, true
}

func (a *App) productShow(w http.ResponseWriter, r *http.Request) {
	product, ok := a.findProduct(w, r)
	if !ok {
		return
	}
	var skus []SKU
	rows, err := a.queries.ListSKUs(r.Context(), product.ID)
	if err != nil {
		a.fail(w, err)
		return
	}
	for _, row := range rows {
		skus = append(skus, skuModel(row))
	}
	a.render(w, r, productviews.Show(a.viewData(w, r, product.Name), product, skus), 200)
}
func (a *App) productForm(w http.ResponseWriter, r *http.Request) {
	p := Product{Status: "draft", TaxRate: 10}
	title := "商品登録"
	if r.PathValue("id") != "" {
		product, ok := a.findProduct(w, r)
		if !ok {
			return
		}
		p = product
		title = "商品編集"
	}
	a.render(w, r, productviews.Form(a.viewData(w, r, title), p, ""), 200)
}
func (a *App) saveProduct(w http.ResponseWriter, r *http.Request) {
	p := Product{}
	if r.PathValue("id") != "" {
		var ok bool
		p, ok = a.findProduct(w, r)
		if !ok {
			return
		}
	}
	p.Name = strings.TrimSpace(r.PostForm.Get("name"))
	p.Description = r.PostForm.Get("description")
	p.Status = r.PostForm.Get("status")
	p.TaxRate, _ = strconv.Atoi(r.PostForm.Get("tax_rate"))
	if err := p.Validate(); err != nil {
		a.render(w, r, productviews.Form(a.viewData(w, r, "商品入力"), p, err.Error()), 422)
		return
	}
	var err error
	if p.ID == 0 {
		p.ID, err = a.queries.CreateProduct(r.Context(), store.CreateProductParams{Name: p.Name, Description: p.Description, Status: p.Status, TaxRate: p.TaxRate})
	} else {
		err = a.queries.UpdateProduct(r.Context(), store.UpdateProductParams{ID: p.ID, Name: p.Name, Description: p.Description, Status: p.Status, TaxRate: p.TaxRate})
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/console/products/%d", p.ID))
}
func (a *App) deleteProduct(w http.ResponseWriter, r *http.Request) {
	p, ok := a.findProduct(w, r)
	if !ok {
		return
	}
	if err := a.queries.DeleteProduct(r.Context(), p.ID); err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/console/products")
}

func (a *App) findSKU(w http.ResponseWriter, r *http.Request, productID int64) (SKU, bool) {
	n, err := strconv.ParseInt(r.PathValue("sku"), 10, 64)
	if err != nil || n <= 0 {
		http.NotFound(w, r)
		return SKU{}, false
	}
	row, err := a.queries.FindSKU(r.Context(), store.FindSKUParams{ID: n, ProductID: productID})
	s := skuModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return s, false
	}
	if err != nil {
		a.fail(w, err)
		return s, false
	}
	return s, true
}
func (a *App) skuForm(w http.ResponseWriter, r *http.Request) {
	p, ok := a.findProduct(w, r)
	if !ok {
		return
	}
	s := SKU{ProductID: p.ID}
	if r.PathValue("sku") != "" {
		s, ok = a.findSKU(w, r, p.ID)
		if !ok {
			return
		}
	}
	a.render(w, r, skuviews.Form(a.viewData(w, r, "SKU入力"), p, s, ""), 200)
}
func (a *App) saveSKU(w http.ResponseWriter, r *http.Request) {
	p, ok := a.findProduct(w, r)
	if !ok {
		return
	}
	s := SKU{ProductID: p.ID}
	if r.PathValue("sku") != "" {
		s, ok = a.findSKU(w, r, p.ID)
		if !ok {
			return
		}
	}
	s.Code = r.PostForm.Get("code")
	s.Size = r.PostForm.Get("size")
	s.Color = r.PostForm.Get("color")
	s.Normalize()
	price, priceErr := strconv.ParseInt(r.PostForm.Get("price_yen"), 10, 64)
	stock, stockErr := strconv.ParseInt(r.PostForm.Get("stock_quantity"), 10, 64)
	s.PriceYen = price
	s.StockQuantity = stock
	err := s.Validate()
	if priceErr != nil || stockErr != nil {
		err = errors.New("価格と在庫は整数で入力してください。")
	}
	if err == nil {
		if s.ID == 0 {
			err = a.queries.CreateSKU(r.Context(), store.CreateSKUParams{ProductID: s.ProductID, Code: s.Code, Size: s.Size, Color: s.Color, PriceYen: s.PriceYen, StockQuantity: s.StockQuantity})
		} else {
			err = a.queries.UpdateSKU(r.Context(), store.UpdateSKUParams{ID: s.ID, ProductID: s.ProductID, Code: s.Code, Size: s.Size, Color: s.Color, PriceYen: s.PriceYen, StockQuantity: s.StockQuantity})
		}
		if err != nil {
			var pgErr interface{ SQLState() string }
			if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
				err = errors.New(dbMessage(err))
			} else {
				a.fail(w, err)
				return
			}
		}
	}
	if err != nil {
		a.render(w, r, skuviews.Form(a.viewData(w, r, "SKU入力"), p, s, err.Error()), 422)
		return
	}
	redirect(w, r, fmt.Sprintf("/console/products/%d", p.ID))
}
func (a *App) deleteSKU(w http.ResponseWriter, r *http.Request) {
	p, ok := a.findProduct(w, r)
	if !ok {
		return
	}
	s, ok := a.findSKU(w, r, p.ID)
	if !ok {
		return
	}
	if err := a.queries.DeleteSKU(r.Context(), store.DeleteSKUParams{ID: s.ID, ProductID: p.ID}); err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, fmt.Sprintf("/console/products/%d", p.ID))
}
