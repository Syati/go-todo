package development

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validAdmins = `password: password123
admins:
  - email: admin@example.com
    role: super_admin
`

const validProducts = `products:
  - name: テスト商品
    status: on_sale
    tax_rate: 10
    skus:
      - code: TEST-01
        size: M
        color: 青
        price_yen: 1000
        stock_quantity: 3
`

func fixtureFS(admins, products string) fstest.MapFS {
	return fstest.MapFS{
		"data/admins.yml":   {Data: []byte(admins)},
		"data/products.yml": {Data: []byte(products)},
	}
}

func TestEmbeddedYAMLFixtures(t *testing.T) {
	data, err := loadData(files)
	require.NoError(t, err)
	assert.Len(t, data.Admins.Admins, 42)
	assert.Equal(t, "password123", data.Admins.Password)
	assert.Equal(t, "admin@example.com", data.Admins.Admins[0].Email)
	assert.Len(t, data.Products, 6)
	total := 0
	for _, product := range data.Products {
		total += len(product.SKUs)
	}
	assert.Equal(t, 13, total)
}

func TestSeedDataNormalization(t *testing.T) {
	admins := strings.ReplaceAll(validAdmins, "admin@example.com", `"  ADMIN@example.com  "`)
	products := strings.ReplaceAll(validProducts, "TEST-01", `" test-01 "`)
	data, err := loadData(fixtureFS(admins, products))
	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", data.Admins.Admins[0].Email)
	assert.Equal(t, "TEST-01", data.Products[0].SKUs[0].Code)
}

func TestRejectInvalidSeedYAML(t *testing.T) {
	for _, tc := range []struct{ name, admins, products, want string }{
		{"syntax", "admins: [", validProducts, "admins.yml"},
		{"unknown admin field", validAdmins + "pasword: typo\n", validProducts, "field pasword"},
		{"unknown SKU field", validAdmins, strings.ReplaceAll(validProducts, "stock_quantity", "stock_quanity"), "field stock_quanity"},
		{"wrong value type", validAdmins, strings.ReplaceAll(validProducts, "price_yen: 1000", "price_yen: invalid"), "products.yml"},
		{"multiple documents", validAdmins + "---\nadmins: []\n", validProducts, "ドキュメントは1つ"},
		{"duplicate YAML key", validAdmins + "password: another\n", validProducts, "already defined"},
		{"missing password", strings.ReplaceAll(validAdmins, "password: password123\n", ""), validProducts, "password"},
		{"invalid email", strings.ReplaceAll(validAdmins, "admin@example.com", "invalid"), validProducts, "メールアドレス"},
		{"invalid role", strings.ReplaceAll(validAdmins, "super_admin", "owner"), validProducts, "role"},
		{"duplicate email", validAdmins + "  - {email: ADMIN@example.com, role: operator}\n", validProducts, "重複"},
		{"duplicate product", validAdmins, validProducts + "  - {name: テスト商品, status: draft, tax_rate: 10}\n", "商品名"},
		{"duplicate SKU code", validAdmins, validProducts + "      - {code: test-01, size: L, color: 赤, price_yen: 0}\n", "SKUコード"},
		{"duplicate variant", validAdmins, validProducts + "      - {code: TEST-02, size: M, color: 青, price_yen: 0}\n", "サイズ・カラー"},
		{"invalid status", validAdmins, strings.ReplaceAll(validProducts, "on_sale", "unknown"), "ステータス"},
		{"invalid tax rate", validAdmins, strings.ReplaceAll(validProducts, "tax_rate: 10", "tax_rate: 9"), "税率"},
		{"negative price", validAdmins, strings.ReplaceAll(validProducts, "price_yen: 1000", "price_yen: -1"), "価格"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadData(fixtureFS(tc.admins, tc.products))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestMissingOrEmptySeedFile(t *testing.T) {
	source := fixtureFS(validAdmins, validProducts)
	delete(source, "data/admins.yml")
	_, err := loadData(source)
	require.ErrorContains(t, err, "admins.yml")
	_, err = loadData(fixtureFS(validAdmins, ""))
	require.ErrorContains(t, err, "products.yml")
}
