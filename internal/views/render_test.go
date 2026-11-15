package views_test

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"go-todo/internal/domain"
	"go-todo/internal/views"
	"go-todo/internal/views/admins"
	"go-todo/internal/views/auth"
	"go-todo/internal/views/components"
	"go-todo/internal/views/products"
	"go-todo/internal/views/skus"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func render(t *testing.T, component templ.Component) (string, *html.Node) {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &out))
	doc, err := html.Parse(strings.NewReader(out.String()))
	require.NoError(t, err)
	return out.String(), doc
}

func elements(root *html.Node, tag string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag {
			found = append(found, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func attribute(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func TestPostFormsKeepActionsAndCSRF(t *testing.T) {
	page := views.Page{Title: "入力", Admin: domain.Admin{ID: 1, Email: "admin@example.test", Role: "super_admin"}, CSRF: "test-csrf-token"}
	for _, tc := range []struct {
		name, action string
		component    templ.Component
	}{
		{"login", "/auth/sign_in", auth.Login(page, "")},
		{"new product", "/console/products", products.Form(page, domain.Product{Status: "draft", TaxRate: 10}, "")},
		{"edit product", "/console/products/42", products.Form(page, domain.Product{ID: 42, Status: "on_sale", TaxRate: 8}, "")},
		{"new SKU", "/console/products/42/skus", skus.Form(page, domain.Product{ID: 42}, domain.SKU{ProductID: 42}, "")},
		{"edit SKU", "/console/products/42/skus/9", skus.Form(page, domain.Product{ID: 42}, domain.SKU{ID: 9, ProductID: 42}, "")},
		{"new admin", "/console/admins", admins.Form(page, domain.Admin{Role: "operator"}, "")},
		{"edit admin", "/console/admins/2", admins.Form(page, domain.Admin{ID: 2, Role: "operator"}, "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, doc := render(t, tc.component)
			found := false
			for _, form := range elements(doc, "form") {
				if attribute(form, "method") != "post" {
					continue
				}
				if attribute(form, "action") == tc.action {
					found = true
				}
				var tokens []string
				for _, input := range elements(form, "input") {
					if attribute(input, "name") == "csrf" {
						tokens = append(tokens, attribute(input, "value"))
					}
				}
				assert.Equal(t, []string{page.CSRF}, tokens, attribute(form, "action"))
			}
			assert.True(t, found, "missing form action %s", tc.action)
		})
	}
}

func TestEscapingAndPaginationPreserveSearch(t *testing.T) {
	query := "机 & 椅子 <script>alert(1)</script>"
	page := views.Page{Title: query}
	data := products.IndexData{Products: []domain.Product{{ID: 1, Name: query, Status: "draft", TaxRate: 10}}, Pagination: components.Pagination{Page: 2, Previous: 1, Next: 3, Query: query, Status: "draft", Sort: "asc"}}
	body, doc := render(t, products.Index(page, data))
	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;")
	var pages []string
	for _, link := range elements(doc, "a") {
		href := attribute(link, "href")
		if !strings.HasPrefix(href, "?") {
			continue
		}
		u, err := url.Parse(href)
		require.NoError(t, err)
		assert.Equal(t, query, u.Query().Get("q"))
		assert.Equal(t, "draft", u.Query().Get("status"))
		assert.Equal(t, "asc", u.Query().Get("sort"))
		pages = append(pages, u.Query().Get("page"))
	}
	assert.Equal(t, []string{"1", "3"}, pages)
}

func TestAdminControlsRespectRoleAndSelf(t *testing.T) {
	for _, tc := range []struct {
		name, role                 string
		self, wantRole, wantDelete bool
	}{
		{"operator", "operator", false, false, false},
		{"self", "super_admin", true, false, false},
		{"other admin", "super_admin", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := views.Page{Admin: domain.Admin{ID: 1, Role: tc.role}}
			target := domain.Admin{ID: 2, Email: "other@example.test", Role: "operator"}
			if tc.self {
				target.ID = 1
			}
			_, show := render(t, admins.Show(page, target))
			hasDelete := false
			for _, form := range elements(show, "form") {
				if strings.HasSuffix(attribute(form, "action"), "/delete") {
					hasDelete = true
				}
			}
			assert.Equal(t, tc.wantDelete, hasDelete)
			// The route blocks operators before rendering a form.
			if tc.role == "super_admin" {
				_, form := render(t, admins.Form(page, target, ""))
				hasRole := false
				for _, selectNode := range elements(form, "select") {
					if attribute(selectNode, "name") == "role" {
						hasRole = true
					}
				}
				assert.Equal(t, tc.wantRole, hasRole)
			}
		})
	}
}
