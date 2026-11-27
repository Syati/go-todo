package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"go-todo/internal/store"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each run owns an isolated schema. No existing application tables are touched.
func testApp(t *testing.T) *App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	ctx := context.Background()
	conn, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	schema := "test_" + randomToken()[:16]
	_, err = conn.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		assert.NoError(t, err)
		assert.NoError(t, conn.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	params := u.Query()
	params.Set("search_path", schema)
	u.RawQuery = params.Encode()
	isolated, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.UpContext(ctx, isolated, "../../db/migrations"))
	// Round trip the initial migration on the isolated, empty schema.
	require.NoError(t, goose.DownContext(ctx, isolated, "../../db/migrations"))
	require.NoError(t, goose.UpContext(ctx, isolated, "../../db/migrations"))
	require.NoError(t, isolated.Close())
	a, err := Open(ctx, u.String(), false)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, a.Close()) })
	require.NoError(t, a.Bootstrap(ctx, "admin@example.test", "test-password-123"))
	return a
}

type browser struct {
	t       *testing.T
	handler http.Handler
	cookies map[string]*http.Cookie
}

func newBrowser(t *testing.T, a *App) *browser {
	return &browser{t: t, handler: a.Echo(), cookies: map[string]*http.Cookie{}}
}
func (b *browser) request(method, path string, form url.Values) *httptest.ResponseRecorder {
	b.t.Helper()
	if form == nil {
		form = url.Values{}
	}
	if method == "POST" {
		if c := b.cookies["csrf"]; c != nil {
			form.Set("csrf", c.Value)
		}
	}
	r := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range b.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	b.handler.ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(b.cookies, cookie.Name)
		} else {
			b.cookies[cookie.Name] = cookie
		}
	}
	return w
}
func (b *browser) login(email string) {
	b.t.Helper()
	require.Equal(b.t, 200, b.request("GET", "/auth/sign_in", nil).Code)
	w := b.request("POST", "/auth/sign_in", url.Values{"email": {email}, "password": {"test-password-123"}})
	require.Equal(b.t, 303, w.Code, w.Body.String())
	require.Equal(b.t, "/", w.Header().Get("Location"))
}

func TestConsoleWithPostgres(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	b := newBrowser(t, a)
	assert.Equal(t, 303, b.request("GET", "/", nil).Code)
	w := b.request("POST", "/console/products", nil)
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/auth/sign_in", w.Header().Get("Location"))
	b.login("admin@example.test")
	assert.Equal(t, 200, b.request("HEAD", "/auth/sign_in", nil).Code)
	assert.Equal(t, 200, b.request("GET", "/", nil).Code)
	for _, path := range []string{"/console/products", "/console/products/new", "/console/admins", "/console/admins/new", "/faq", "/up"} {
		assert.Equal(t, 200, b.request("GET", path, nil).Code, path)
	}
	w = b.request("POST", "/console/products", url.Values{"name": {"<script>alert(1)</script> 椅子"}, "status": {"on_sale"}, "tax_rate": {"10"}})
	require.Equal(t, 303, w.Code, w.Body.String())
	productPath := w.Header().Get("Location")
	w = b.request("GET", productPath, nil)
	assert.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>alert(1)</script>")
	assert.Contains(t, w.Body.String(), "&lt;script&gt;")
	assert.Equal(t, 200, b.request("GET", productPath+"/edit", nil).Code)
	assert.Equal(t, 200, b.request("GET", productPath+"/skus/new", nil).Code)
	w = b.request("POST", productPath, nil)
	assert.Equal(t, 422, w.Code)
	w = b.request("POST", productPath, url.Values{"name": {"更新した椅子"}, "status": {"draft"}, "tax_rate": {"8"}})
	assert.Equal(t, 303, w.Code)
	skuForm := url.Values{"code": {" chair-01 "}, "size": {" M "}, "color": {" 青 "}, "price_yen": {"12000"}, "stock_quantity": {"3"}}
	w = b.request("POST", productPath+"/skus", skuForm)
	require.Equal(t, 303, w.Code, w.Body.String())
	w = b.request("POST", productPath+"/skus", skuForm)
	assert.Equal(t, 422, w.Code, "duplicate SKU code")
	skuForm.Set("code", "CHAIR-02")
	w = b.request("POST", productPath+"/skus", skuForm)
	assert.Equal(t, 422, w.Code, "duplicate normalized variant")
	products, err := a.queries.ListProducts(ctx, store.ListProductsParams{Search: "更新"})
	require.NoError(t, err)
	require.Len(t, products, 1)
	skus, err := a.queries.ListSKUs(ctx, products[0].ID)
	require.NoError(t, err)
	require.Len(t, skus, 1)
	assert.Equal(t, "CHAIR-01", skus[0].Code)
	skuPath := fmt.Sprintf("%s/skus/%d", productPath, skus[0].ID)
	assert.Equal(t, 200, b.request("GET", skuPath+"/edit", nil).Code)
	skuForm.Set("code", "CHAIR-01")
	skuForm.Set("stock_quantity", "7")
	assert.Equal(t, 303, b.request("POST", skuPath, skuForm).Code)
	// A SKU must belong to the product in the path.
	other, err := a.queries.CreateProduct(ctx, store.CreateProductParams{Name: "別商品", Status: "draft", TaxRate: 10})
	require.NoError(t, err)
	assert.Equal(t, 404, b.request("POST", fmt.Sprintf("/console/products/%d/skus/%d/delete", other, skus[0].ID), nil).Code)
	assert.Equal(t, 303, b.request("POST", skuPath+"/delete", nil).Code)
	assert.Equal(t, 303, b.request("POST", productPath+"/delete", nil).Code)
	assert.Equal(t, 404, b.request("GET", productPath, nil).Code)

	// New admins always start as operators, even if the form is tampered with.
	w = b.request("POST", "/console/admins", url.Values{"email": {"operator@example.test"}, "password": {"test-password-123"}, "password_confirmation": {"test-password-123"}, "role": {"super_admin"}})
	require.Equal(t, 303, w.Code, w.Body.String())
	adminPath := w.Header().Get("Location")
	operator, err := a.queries.FindLoginAdmin(ctx, "operator@example.test")
	require.NoError(t, err)
	assert.Equal(t, "operator", operator.Role)
	assert.Equal(t, 200, b.request("GET", adminPath, nil).Code)
	assert.Equal(t, 200, b.request("GET", adminPath+"/edit", nil).Code)
	op := newBrowser(t, a)
	op.login(operator.Email)
	for _, path := range []string{"/console/admins/new", adminPath + "/edit"} {
		assert.Equal(t, 403, op.request("GET", path, nil).Code)
	}
	assert.Equal(t, 403, op.request("POST", adminPath+"/delete", nil).Code)
	assert.Equal(t, 200, op.request("GET", "/console/products", nil).Code)
	current, err := a.queries.FindLoginAdmin(ctx, "admin@example.test")
	require.NoError(t, err)
	assert.Equal(t, 422, b.request("POST", fmt.Sprintf("/console/admins/%d/delete", current.ID), nil).Code)
	// Bootstrap does not create a second super admin or reset credentials.
	require.NoError(t, a.Bootstrap(ctx, "second@example.test", "test-password-456"))
	_, err = a.queries.FindLoginAdmin(ctx, "second@example.test")
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Equal(t, 303, b.request("POST", adminPath+"/delete", nil).Code)
	assert.Equal(t, 303, op.request("GET", "/", nil).Code, "discarded admin loses access immediately")
	assert.Equal(t, 303, b.request("POST", "/auth/sign_out", nil).Code)
	assert.Equal(t, 303, b.request("GET", "/", nil).Code)
}

func TestLoginCSRFAndRateLimit(t *testing.T) {
	a := testApp(t)
	b := newBrowser(t, a)
	b.request("GET", "/auth/sign_in", nil)
	for i := 0; i < 5; i++ {
		assert.Equal(t, 422, b.request("POST", "/auth/sign_in", url.Values{"email": {"missing@example.test"}, "password": {"wrong"}}).Code)
	}
	assert.Equal(t, 429, b.request("POST", "/auth/sign_in", url.Values{"email": {"missing@example.test"}, "password": {"wrong"}}).Code)
	r := httptest.NewRequest("POST", "/auth/sign_in", strings.NewReader("csrf=wrong"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(b.cookies["csrf"])
	w := httptest.NewRecorder()
	b.handler.ServeHTTP(w, r)
	assert.Equal(t, 403, w.Code)
	r = httptest.NewRequest("POST", "/auth/sign_in", strings.NewReader(url.Values{"csrf": {b.cookies["csrf"].Value}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://untrusted.example")
	r.AddCookie(b.cookies["csrf"])
	w = httptest.NewRecorder()
	b.handler.ServeHTTP(w, r)
	assert.Equal(t, 403, w.Code)
}
