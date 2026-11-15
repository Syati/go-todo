package seeds_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"go-todo/db/migrations"
	"go-todo/db/seeds"
	"go-todo/internal/cli"
	"go-todo/internal/store"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func isolatedDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	conn, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	schema := "seed_" + strings.ToLower(rand.Text()[:16])
	_, err = conn.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.Exec("DROP SCHEMA " + schema + " CASCADE")
		assert.NoError(t, err)
		assert.NoError(t, conn.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	params := u.Query()
	params.Set("search_path", schema)
	u.RawQuery = params.Encode()
	db, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
	return db, u.String()
}

func TestUndefinedEnvironmentDoesNothing(t *testing.T) {
	for _, environment := range []string{"test", "production", "custom"} {
		t.Run(environment, func(t *testing.T) {
			applied, err := seeds.Run(context.Background(), nil, environment)
			require.NoError(t, err)
			assert.False(t, applied)
		})
	}
}

func TestDevelopmentSeedsFromCLI(t *testing.T) {
	db, dsn := isolatedDB(t)
	ctx := context.Background()
	queries := store.New(db)
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("BOOTSTRAP_EMAIL", "")
	t.Setenv("BOOTSTRAP_PASSWORD", "")
	t.Chdir(t.TempDir())
	run := func() {
		t.Helper()
		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--env", "development", "db", "seed"})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, out.String(), "seed 完了")
		assert.NotContains(t, out.String(), "password123")
	}
	run()
	admin, err := queries.FindLoginAdmin(ctx, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, "super_admin", admin.Role)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("password123")))
	assertCounts := func(deleted int) {
		t.Helper()
		var adminCount, deletedCount, skuCount int
		require.NoError(t, db.QueryRow("SELECT count(*), count(deleted_at) FROM admins").Scan(&adminCount, &deletedCount))
		require.NoError(t, db.QueryRow("SELECT count(*) FROM catalog_skus").Scan(&skuCount))
		count, err := queries.CountProducts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 42, adminCount)
		assert.Equal(t, deleted, deletedCount)
		assert.EqualValues(t, 6, count)
		assert.Equal(t, 13, skuCount)
	}
	assertCounts(1)
	productID, err := queries.FindSeedProduct(ctx, "オーガニックコットン T シャツ")
	require.NoError(t, err)
	skus, err := queries.ListSKUs(ctx, productID)
	require.NoError(t, err)
	require.Len(t, skus, 6)
	assert.Equal(t, "TS-ORG-01", skus[0].Code)
	assert.EqualValues(t, 3300, skus[0].PriceYen)
	assert.EqualValues(t, 12, skus[0].StockQuantity)
	teaID, err := queries.FindSeedProduct(ctx, "有機緑茶 100g")
	require.NoError(t, err)
	tea, err := queries.FindProduct(ctx, teaID)
	require.NoError(t, err)
	assert.Equal(t, 8, tea.TaxRate)

	// Seeding must not reset edits, passwords, permissions, or soft deletions.
	hash, err := bcrypt.GenerateFromPassword([]byte("changed-password-123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, queries.UpdateAdmin(ctx, store.UpdateAdminParams{ID: admin.ID, Email: admin.Email, Role: "operator", NewPasswordHash: string(hash)}))
	require.NoError(t, queries.UpdateProduct(ctx, store.UpdateProductParams{ID: productID, Name: "オーガニックコットン T シャツ", Description: "編集した説明", Status: "draft", TaxRate: 8}))
	sku := skus[0]
	require.NoError(t, queries.UpdateSKU(ctx, store.UpdateSKUParams{ID: sku.ID, ProductID: productID, Code: sku.Code, Size: sku.Size, Color: sku.Color, PriceYen: 5000, StockQuantity: 99}))
	operatorID, err := queries.FindSeedAdmin(ctx, "operator01@example.com")
	require.NoError(t, err)
	require.NoError(t, queries.DiscardAdmin(ctx, operatorID))
	run()
	assertCounts(2)
	admin, err = queries.FindLoginAdmin(ctx, admin.Email)
	require.NoError(t, err)
	assert.Equal(t, "operator", admin.Role)
	assert.Equal(t, string(hash), admin.PasswordHash)
	product, err := queries.FindProduct(ctx, productID)
	require.NoError(t, err)
	assert.Equal(t, "編集した説明", product.Description)
	assert.Equal(t, "draft", product.Status)
	assert.Equal(t, 8, product.TaxRate)
	updatedSKU, err := queries.FindSKU(ctx, store.FindSKUParams{ID: sku.ID, ProductID: productID})
	require.NoError(t, err)
	assert.EqualValues(t, 5000, updatedSKU.PriceYen)
	assert.EqualValues(t, 99, updatedSKU.StockQuantity)
}

func TestSeedFailureRollsBackAllSeedChanges(t *testing.T) {
	db, _ := isolatedDB(t)
	ctx := context.Background()
	queries := store.New(db)
	productID, err := queries.CreateProduct(ctx, store.CreateProductParams{Name: "オーガニックコットン T シャツ", Status: "draft", TaxRate: 10})
	require.NoError(t, err)
	require.NoError(t, queries.CreateSKU(ctx, store.CreateSKUParams{ProductID: productID, Code: "EXISTING", Size: "S", Color: "ホワイト", PriceYen: 1000}))
	_, err = seeds.Run(ctx, db, "development")
	require.Error(t, err)
	var adminCount int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM admins").Scan(&adminCount))
	assert.Zero(t, adminCount)
	count, err := queries.CountProducts(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
