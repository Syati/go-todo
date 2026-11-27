package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-todo/internal/i18n"
	"go-todo/internal/store"
	"go-todo/internal/views"
	authviews "go-todo/internal/views/auth"
	dashboardviews "go-todo/internal/views/dashboard"
	faqviews "go-todo/internal/views/faq"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db           *sql.DB
	queries      *store.Queries
	secure       bool
	dummyHash    []byte
	translations *i18n.Bundle
	router       *echo.Echo
}

func Open(ctx context.Context, url string, secure bool) (*App, error) {
	if url == "" {
		return nil, errors.New("DATABASE_URL を指定してください。README.md を参照してください。")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	setup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err = db.PingContext(setup); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte(randomToken()), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	translations, err := i18n.NewBundle()
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	application := &App{db: db, queries: store.New(db), secure: secure, dummyHash: dummy, translations: translations}
	application.router = application.routes()
	return application, nil
}

func (a *App) Close() error { return a.db.Close() }

func (a *App) Bootstrap(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validateEmail(email); err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	// Bootstrap only an empty database; never elevate or overwrite an existing account.
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // Commit 後の ErrTxDone は無視する。
	q := a.queries.WithTx(tx)
	if err = q.LockAdmins(ctx); err != nil {
		return err
	}
	err = q.BootstrapAdmin(ctx, store.BootstrapAdminParams{Email: email, PasswordHash: string(hash)})
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) Echo() *echo.Echo { return a.router }

func (a *App) viewData(c *echo.Context, title string) views.Page {
	localizer := a.localizer(c)
	p := views.Page{Title: localizer.Message(title), Locale: localizer.Language(), T: localizer.Message}
	if admin, ok := c.Get(adminContextKey).(*Admin); ok {
		p.Admin = *admin
	}
	p.CSRF, _ = c.Get("csrf").(string)
	return p
}

func (a *App) render(c *echo.Context, component templ.Component, status int) error {
	var out strings.Builder
	if err := component.Render(c.Request().Context(), &out); err != nil {
		return err
	}
	return c.HTMLBlob(status, []byte(out.String()))
}

func (a *App) cookie(c *echo.Context, name, value string, age int) {
	c.SetCookie(&http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func tokenHash(s string) string { hash := sha256.Sum256([]byte(s)); return hex.EncodeToString(hash[:]) }
func (a *App) message(c *echo.Context, id string) string {
	return a.localizer(c).Message(id)
}
func (a *App) inputError(c *echo.Context, err error) string {
	keys := map[string]string{
		"メールアドレスを正しく入力してください。":                     "error.email_invalid",
		"パスワードは12〜72バイトで入力してください。":                 "error.password_length",
		"権限が不正です。":                                 "error.role_invalid",
		"確認用パスワードが一致しません。":                         "error.password_mismatch",
		"商品名は1〜100文字で入力してください。":                    "error.product_name",
		"説明は2000文字以内で入力してください。":                    "error.product_description",
		"商品ステータスが不正です。":                            "error.product_status",
		"税率は8%または10%を指定してください。":                    "error.product_tax",
		"SKUコードは50文字以内の英数字・ハイフンで入力してください（先頭は英数字）。": "error.sku_code",
		"サイズ・カラーは30文字以内で入力してください。":                 "error.sku_variant",
		"価格は0〜10,000,000円、在庫は0以上の整数で入力してください。":     "error.sku_price_stock",
		"価格と在庫は整数で入力してください。":                       "error.sku_integer",
		"メールアドレス、SKUコード、またはサイズとカラーの組み合わせが重複しています。": "error.duplicate",
	}
	key, ok := keys[err.Error()]
	if !ok {
		key = "error.invalid_input"
	}
	return a.message(c, key)
}
func id(c *echo.Context) (int64, error) {
	n, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err == nil && n <= 0 {
		err = errors.New("invalid id")
	}
	return n, err
}
func redirect(c *echo.Context, path string) error {
	return c.Redirect(http.StatusSeeOther, path)
}
func dbMessage(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return "メールアドレス、SKUコード、またはサイズとカラーの組み合わせが重複しています。"
	}
	return "入力内容を確認してください。"
}

func (a *App) login(c *echo.Context) error {
	if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead {
		return a.render(c, authviews.Login(a.viewData(c, "title.login"), ""), http.StatusOK)
	}
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	ctx := c.Request().Context()
	identityHash := tokenHash(email)
	expiresAt, err := a.queries.ReserveLoginAttempt(ctx, identityHash)
	if errors.Is(err, sql.ErrNoRows) {
		return a.render(c, authviews.Login(a.viewData(c, "title.login"), a.message(c, "error.login_throttled")), http.StatusTooManyRequests)
	}
	if err != nil {
		return err
	}
	admin, err := a.queries.FindLoginAdmin(ctx, email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	compare := []byte(admin.PasswordHash)
	if admin.PasswordHash == "" {
		compare = a.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword(compare, []byte(c.FormValue("password")))
	if err != nil || passwordErr != nil {
		return a.render(c, authviews.Login(a.viewData(c, "title.login"), a.message(c, "error.login_invalid")), http.StatusUnprocessableEntity)
	}
	token := randomToken()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := a.queries.WithTx(tx)
	// A password reset or account deletion may have committed during bcrypt.
	_, err = q.LockLoginAdminCredentials(ctx, store.LockLoginAdminCredentialsParams{ID: admin.ID, PasswordHash: admin.PasswordHash})
	if errors.Is(err, sql.ErrNoRows) {
		return a.render(c, authviews.Login(a.viewData(c, "title.login"), a.message(c, "error.login_invalid")), http.StatusUnprocessableEntity)
	}
	if err != nil {
		return err
	}
	if err := q.CreateSession(ctx, store.CreateSessionParams{TokenHash: tokenHash(token), AdminID: admin.ID}); err != nil {
		return err
	}
	if err := q.ResetLoginAttempts(ctx, store.ResetLoginAttemptsParams{IdentityHash: identityHash, ExpiresAt: expiresAt}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.cookie(c, "session", token, 12*3600)
	a.cookie(c, "csrf", randomToken(), 0)
	return redirect(c, "/")
}
func (a *App) logout(c *echo.Context) error {
	cookie, _ := c.Cookie("session")
	if err := a.queries.DeleteSession(c.Request().Context(), tokenHash(cookie.Value)); err != nil {
		return err
	}
	a.cookie(c, "session", "", -1)
	a.cookie(c, "csrf", "", -1)
	return redirect(c, "/auth/sign_in")
}
func (a *App) dashboard(c *echo.Context) error {
	count, err := a.queries.CountProducts(c.Request().Context())
	if err != nil {
		return err
	}
	return a.render(c, dashboardviews.Index(a.viewData(c, "title.dashboard"), count), http.StatusOK)
}
func (a *App) faq(c *echo.Context) error {
	return a.render(c, faqviews.Index(a.viewData(c, "title.faq")), http.StatusOK)
}
