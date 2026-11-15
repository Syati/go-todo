package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "ADDR", "DATABASE_URL", "COOKIE_SECURE", "BOOTSTRAP_EMAIL", "BOOTSTRAP_PASSWORD"} {
		t.Setenv(key, "")
	}
}
func writeConfig(t *testing.T, dir, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".yml"), []byte(contents), 0600))
}

func TestConfigEnvironmentAndPrecedence(t *testing.T) {
	cleanEnvironment(t)
	dir := t.TempDir()
	writeConfig(t, dir, "development", "database_url: postgres://dev@localhost/test\naddr: localhost:9000\n")
	writeConfig(t, dir, "production", "addr: 0.0.0.0:8080\ncookie_secure: true\n")
	cfg, err := Load(Options{Directory: dir})
	require.NoError(t, err)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "localhost:9000", cfg.Addr)
	assert.False(t, cfg.CookieSecure)
	writeConfig(t, dir, "development.local", "addr: localhost:9500\ncookie_secure: true\n")
	cfg, err = Load(Options{Directory: dir})
	require.NoError(t, err)
	assert.Equal(t, "localhost:9500", cfg.Addr)
	assert.True(t, cfg.CookieSecure)
	t.Setenv("APP_ENV", "production")
	_, err = Load(Options{Directory: dir})
	require.Error(t, err, "production must not inherit development's database URL")
	t.Setenv("DATABASE_URL", "postgres://production@localhost/test")
	cfg, err = Load(Options{Directory: dir})
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.Environment)
	assert.True(t, cfg.CookieSecure)
	assert.Equal(t, "0.0.0.0:8080", cfg.Addr)
	t.Setenv("ADDR", "127.0.0.1:9090")
	t.Setenv("COOKIE_SECURE", "false")
	cfg, err = Load(Options{Directory: dir, Environment: "development"})
	require.NoError(t, err)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "127.0.0.1:9090", cfg.Addr)
	assert.False(t, cfg.CookieSecure)
	assert.Equal(t, "postgres://production@localhost/test", cfg.DatabaseURL)
}

func TestConfigRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct{ name, contents string }{
		{"missing database", "addr: localhost:8000\n"},
		{"unknown key", "database_url: postgres://localhost/test\ncokie_secure: true\n"},
		{"invalid bool", "database_url: postgres://localhost/test\ncookie_secure: maybe\n"},
		{"invalid url", "database_url: mysql://localhost/test\n"},
		{"invalid addr", "database_url: postgres://localhost/test\naddr: wrong\n"},
		{"partial bootstrap", "database_url: postgres://localhost/test\nbootstrap_email: admin@example.test\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanEnvironment(t)
			dir := t.TempDir()
			writeConfig(t, dir, "development", tc.contents)
			_, err := Load(Options{Directory: dir})
			require.Error(t, err)
		})
	}
}

func TestMissingEnvironmentAndPathTraversal(t *testing.T) {
	cleanEnvironment(t)
	dir := t.TempDir()
	_, err := Load(Options{Directory: dir, Environment: "missing"})
	require.Error(t, err)
	_, err = Load(Options{Directory: dir, Environment: "../production"})
	require.Error(t, err)
	_, err = Load(Options{Directory: t.TempDir()})
	require.Error(t, err)
}

func TestLocalConfigErrorsAreNotIgnored(t *testing.T) {
	for _, contents := range []string{"addr: [broken", "cokie_secure: true\n", "cookie_secure: maybe\n"} {
		t.Run(contents, func(t *testing.T) {
			cleanEnvironment(t)
			dir := t.TempDir()
			writeConfig(t, dir, "development", "database_url: postgres://localhost/test\n")
			writeConfig(t, dir, "development.local", contents)
			_, err := Load(Options{Directory: dir})
			require.Error(t, err)
		})
	}
}

func TestFileDefaultsWithoutLocalConfig(t *testing.T) {
	cleanEnvironment(t)
	dir := t.TempDir()
	writeConfig(t, dir, "development", "database_url: postgres://localhost/test\n")
	cfg, err := Load(Options{Directory: dir})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8080", cfg.Addr)
	assert.False(t, cfg.CookieSecure)
}

func TestEmbeddedSettingsOutsideRepository(t *testing.T) {
	cleanEnvironment(t)
	t.Chdir(t.TempDir())
	cfg, err := Load(Options{})
	require.NoError(t, err)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "127.0.0.1:8080", cfg.Addr)
	assert.NotEmpty(t, cfg.DatabaseURL)
	t.Setenv("APP_ENV", "production")
	_, err = Load(Options{})
	require.Error(t, err, "production still requires DATABASE_URL")
	t.Setenv("DATABASE_URL", "postgres://production@localhost/test")
	cfg, err = Load(Options{})
	require.NoError(t, err)
	assert.True(t, cfg.CookieSecure)
	assert.Equal(t, "0.0.0.0:8080", cfg.Addr)
}

func TestLocalOverrideOnEmbeddedSettings(t *testing.T) {
	cleanEnvironment(t)
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Join("config", "settings"), 0755))
	writeConfig(t, filepath.Join("config", "settings"), "development.local", "addr: localhost:9090\n")
	cfg, err := Load(Options{})
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", cfg.Addr)
	t.Setenv("ADDR", "localhost:9091")
	cfg, err = Load(Options{})
	require.NoError(t, err)
	assert.Equal(t, "localhost:9091", cfg.Addr)
}
