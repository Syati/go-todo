package cli

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedMigrationsOutsideRepository(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	conn, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	schema := "cli_" + rand.Text()[:16]
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
	t.Setenv("DATABASE_URL", u.String())
	t.Setenv("BOOTSTRAP_EMAIL", "")
	t.Setenv("BOOTSTRAP_PASSWORD", "")
	t.Chdir(t.TempDir())
	run := func(operation string) string {
		t.Helper()
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--env", "test", "db", operation})
		require.NoError(t, cmd.Execute())
		return out.String()
	}
	assert.Contains(t, run("status"), "pending")
	assert.Contains(t, run("up"), "00001_initial.sql")
	assert.Contains(t, run("status"), "applied")
	assert.Empty(t, run("up"), "applying already-applied migrations is a no-op")
	assert.Contains(t, run("down"), "00001_initial.sql")
	assert.Contains(t, run("status"), "pending")
}
