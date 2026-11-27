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

	"github.com/labstack/echo/v5"
)

func pagination(page string) (int, int) {
	n, _ := strconv.Atoi(page)
	if n < 1 || n > 1000000 {
		n = 1
	}
	return n, (n - 1) * 30
}

func (a *App) products(c *echo.Context) error {
	n, offset := pagination(c.QueryParam("page"))
	p := productviews.IndexData{Pagination: components.Pagination{Query: c.QueryParam("q"), Status: c.QueryParam("status"), Sort: c.QueryParam("sort"), Page: n}}
	rows, err := a.queries.ListProducts(c.Request().Context(), store.ListProductsParams{Search: p.Query, StatusFilter: p.Status, Ascending: p.Sort == "asc", PageOffset: offset})
	if err != nil {
		return err
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
	return a.render(c, productviews.Index(a.viewData(c, "title.products"), p), http.StatusOK)
}

func (a *App) findProduct(c *echo.Context) (Product, error) {
	n, err := id(c)
	if err != nil {
		return Product{}, echo.ErrNotFound
	}
	row, err := a.queries.FindProduct(c.Request().Context(), n)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, echo.ErrNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return productModel(row), nil
}

func (a *App) productShow(c *echo.Context) error {
	product, err := a.findProduct(c)
	if err != nil {
		return err
	}
	var skus []SKU
	rows, err := a.queries.ListSKUs(c.Request().Context(), product.ID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		skus = append(skus, skuModel(row))
	}
	return a.render(c, productviews.Show(a.viewData(c, product.Name), product, skus), http.StatusOK)
}

func (a *App) productForm(c *echo.Context) error {
	p := Product{Status: "draft", TaxRate: 10}
	title := "title.product_new"
	if c.Param("id") != "" {
		product, err := a.findProduct(c)
		if err != nil {
			return err
		}
		p = product
		title = "title.product_edit"
	}
	return a.render(c, productviews.Form(a.viewData(c, title), p, ""), http.StatusOK)
}

func (a *App) saveProduct(c *echo.Context) error {
	p := Product{}
	if c.Param("id") != "" {
		var err error
		p, err = a.findProduct(c)
		if err != nil {
			return err
		}
	}
	p.Name = strings.TrimSpace(c.FormValue("name"))
	p.Description = c.FormValue("description")
	p.Status = c.FormValue("status")
	p.TaxRate, _ = strconv.Atoi(c.FormValue("tax_rate"))
	if err := p.Validate(); err != nil {
		return a.render(c, productviews.Form(a.viewData(c, "title.product_form"), p, a.inputError(c, err)), http.StatusUnprocessableEntity)
	}
	var err error
	if p.ID == 0 {
		p.ID, err = a.queries.CreateProduct(c.Request().Context(), store.CreateProductParams{Name: p.Name, Description: p.Description, Status: p.Status, TaxRate: p.TaxRate})
	} else {
		err = a.auditedMutation(c, func(q *store.Queries) error {
			return q.UpdateProduct(c.Request().Context(), store.UpdateProductParams{ID: p.ID, Name: p.Name, Description: p.Description, Status: p.Status, TaxRate: p.TaxRate})
		})
	}
	if err != nil {
		return err
	}
	return redirect(c, fmt.Sprintf("/console/products/%d", p.ID))
}

func (a *App) deleteProduct(c *echo.Context) error {
	p, err := a.findProduct(c)
	if err != nil {
		return err
	}
	if err := a.auditedMutation(c, func(q *store.Queries) error {
		return q.DeleteProduct(c.Request().Context(), p.ID)
	}); err != nil {
		return err
	}
	return redirect(c, "/console/products")
}

func (a *App) findSKU(c *echo.Context, productID int64) (SKU, error) {
	n, err := strconv.ParseInt(c.Param("sku"), 10, 64)
	if err != nil || n <= 0 {
		return SKU{}, echo.ErrNotFound
	}
	row, err := a.queries.FindSKU(c.Request().Context(), store.FindSKUParams{ID: n, ProductID: productID})
	if errors.Is(err, sql.ErrNoRows) {
		return SKU{}, echo.ErrNotFound
	}
	if err != nil {
		return SKU{}, err
	}
	return skuModel(row), nil
}

func (a *App) skuForm(c *echo.Context) error {
	p, err := a.findProduct(c)
	if err != nil {
		return err
	}
	s := SKU{ProductID: p.ID}
	if c.Param("sku") != "" {
		s, err = a.findSKU(c, p.ID)
		if err != nil {
			return err
		}
	}
	return a.render(c, skuviews.Form(a.viewData(c, "title.sku_form"), p, s, ""), http.StatusOK)
}

func (a *App) saveSKU(c *echo.Context) error {
	p, err := a.findProduct(c)
	if err != nil {
		return err
	}
	s := SKU{ProductID: p.ID}
	if c.Param("sku") != "" {
		s, err = a.findSKU(c, p.ID)
		if err != nil {
			return err
		}
	}
	s.Code = c.FormValue("code")
	s.Size = c.FormValue("size")
	s.Color = c.FormValue("color")
	s.Normalize()
	price, priceErr := strconv.ParseInt(c.FormValue("price_yen"), 10, 64)
	stock, stockErr := strconv.ParseInt(c.FormValue("stock_quantity"), 10, 64)
	s.PriceYen = price
	s.StockQuantity = stock
	err = s.Validate()
	if priceErr != nil || stockErr != nil {
		err = errors.New("価格と在庫は整数で入力してください。")
	}
	if err == nil {
		if s.ID == 0 {
			err = a.queries.CreateSKU(c.Request().Context(), store.CreateSKUParams{ProductID: s.ProductID, Code: s.Code, Size: s.Size, Color: s.Color, PriceYen: s.PriceYen, StockQuantity: s.StockQuantity})
		} else {
			err = a.auditedMutation(c, func(q *store.Queries) error {
				return q.UpdateSKU(c.Request().Context(), store.UpdateSKUParams{ID: s.ID, ProductID: s.ProductID, Code: s.Code, Size: s.Size, Color: s.Color, PriceYen: s.PriceYen, StockQuantity: s.StockQuantity})
			})
		}
		if err != nil {
			var pgErr interface{ SQLState() string }
			if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
				err = errors.New(dbMessage(err))
			} else {
				return err
			}
		}
	}
	if err != nil {
		return a.render(c, skuviews.Form(a.viewData(c, "title.sku_form"), p, s, a.inputError(c, err)), http.StatusUnprocessableEntity)
	}
	return redirect(c, fmt.Sprintf("/console/products/%d", p.ID))
}

func (a *App) deleteSKU(c *echo.Context) error {
	p, err := a.findProduct(c)
	if err != nil {
		return err
	}
	s, err := a.findSKU(c, p.ID)
	if err != nil {
		return err
	}
	if err := a.auditedMutation(c, func(q *store.Queries) error {
		return q.DeleteSKU(c.Request().Context(), store.DeleteSKUParams{ID: s.ID, ProductID: p.ID})
	}); err != nil {
		return err
	}
	return redirect(c, fmt.Sprintf("/console/products/%d", p.ID))
}
