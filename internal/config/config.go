package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	settings "go-todo/config"

	"github.com/spf13/viper"
)

type Config struct {
	Environment       string `mapstructure:"app_env"`
	Addr              string `mapstructure:"addr"`
	DatabaseURL       string `mapstructure:"database_url"`
	CookieSecure      bool   `mapstructure:"cookie_secure"`
	BootstrapEmail    string `mapstructure:"bootstrap_email"`
	BootstrapPassword string `mapstructure:"bootstrap_password"`
}

type Options struct{ Environment, Directory string }

// Load reads external settings, falling back to embedded defaults when no
// directory is explicitly selected. Local overrides and environment variables
// take precedence. The CLI environment overrides APP_ENV.
func Load(options Options) (Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetDefault("app_env", "development")
	v.SetDefault("addr", "127.0.0.1:8080")
	v.SetDefault("cookie_secure", false)
	for _, key := range []string{"app_env", "addr", "database_url", "cookie_secure", "bootstrap_email", "bootstrap_password"} {
		if err := v.BindEnv(key); err != nil {
			return Config{}, err
		}
	}
	if options.Environment != "" {
		v.Set("app_env", options.Environment)
	}
	environment := v.GetString("app_env")
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(environment) {
		return Config{}, errors.New("環境名は英小文字で始まる英小文字・数字・ハイフン・アンダースコアを指定してください")
	}
	v.Set("app_env", environment)
	directory := options.Directory
	if directory == "" {
		directory = filepath.Join("config", "settings")
	}
	v.SetConfigFile(filepath.Join(directory, environment+".yml"))
	if err := v.ReadInConfig(); err != nil {
		if options.Directory != "" || !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("環境 %s の設定を読み込めません: %w", environment, err)
		}
		data, err := settings.Files.ReadFile("settings/" + environment + ".yml")
		if err != nil {
			return Config{}, fmt.Errorf("環境 %s の埋め込み設定を読み込めません: %w", environment, err)
		}
		if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
			return Config{}, fmt.Errorf("環境 %s の埋め込み設定が不正です: %w", environment, err)
		}
	}
	v.SetConfigFile(filepath.Join(directory, environment+".local.yml"))
	if err := v.MergeInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("環境 %s のローカル設定を読み込めません: %w", environment, err)
	}
	var c Config
	if err := v.UnmarshalExact(&c); err != nil {
		return Config{}, fmt.Errorf("設定値が不正です: %w", err)
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL または環境設定の database_url を指定してください")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return Config{}, errors.New("database_url は PostgreSQL の接続URLを指定してください")
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return Config{}, errors.New("addr は host:port 形式で指定してください")
	}
	if (c.BootstrapEmail == "") != (c.BootstrapPassword == "") {
		return Config{}, errors.New("bootstrap_email と bootstrap_password は両方指定してください")
	}
	return c, nil
}
