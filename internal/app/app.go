package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db           *sql.DB
	queries      *store.Queries
	secure       bool
	mu           sync.Mutex
	attempts     map[string]attempt
	dummyHash    []byte
	translations *i18n.Bundle
}
type attempt struct {
	Count int
	Until time.Time
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
	return &App{db: db, queries: store.New(db), secure: secure, attempts: map[string]attempt{}, dummyHash: dummy, translations: translations}, nil
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

func (a *App) Handler() http.Handler {
	return a.routes()
}

func (a *App) viewData(w http.ResponseWriter, r *http.Request, title string) views.Page {
	localizer := a.localizer(r)
	p := views.Page{Title: localizer.Message(title), Locale: localizer.Language(), T: localizer.Message}
	if admin, ok := r.Context().Value(adminContextKey{}).(*Admin); ok {
		p.Admin = *admin
	}
	cookie, err := r.Cookie("csrf")
	if err != nil || len(cookie.Value) != 64 {
		p.CSRF = randomToken()
		a.cookie(w, "csrf", p.CSRF, 0)
	} else {
		p.CSRF = cookie.Value
	}
	return p
}

func (a *App) render(w http.ResponseWriter, r *http.Request, component templ.Component, status int) {
	var out strings.Builder
	if err := component.Render(r.Context(), &out); err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := fmt.Fprint(w, out.String()); err != nil {
		slog.Warn("response write failed", "error", err)
	}
}

func (a *App) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func tokenHash(s string) string { hash := sha256.Sum256([]byte(s)); return hex.EncodeToString(hash[:]) }
func (a *App) message(r *http.Request, id string) string {
	return a.localizer(r).Message(id)
}
func (a *App) inputError(r *http.Request, err error) string {
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
	return a.message(r, key)
}
func (a *App) fail(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "error", err)
	http.Error(w, a.message(r, "error.internal"), 500)
}
func id(r *http.Request) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil && n <= 0 {
		err = errors.New("invalid id")
	}
	return n, err
}
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}
func dbMessage(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return "メールアドレス、SKUコード、またはサイズとカラーの組み合わせが重複しています。"
	}
	return "入力内容を確認してください。"
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" || r.Method == "HEAD" {
		a.render(w, r, authviews.Login(a.viewData(w, r, "title.login"), ""), 200)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.PostForm.Get("email")))
	// Bound the in-process limiter, including attempts for unknown accounts.
	a.mu.Lock()
	now := time.Now()
	for key, item := range a.attempts {
		if now.After(item.Until) {
			delete(a.attempts, key)
		}
	}
	item := a.attempts[email]
	if item.Count >= 5 || (len(a.attempts) >= 10000 && item.Count == 0) {
		a.mu.Unlock()
		a.render(w, r, authviews.Login(a.viewData(w, r, "title.login"), a.message(r, "error.login_throttled")), 429)
		return
	}
	item.Count++
	if item.Until.IsZero() {
		item.Until = now.Add(15 * time.Minute)
	}
	a.attempts[email] = item
	a.mu.Unlock()
	admin, err := a.queries.FindLoginAdmin(r.Context(), email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.fail(w, r, err)
		return
	}
	compare := []byte(admin.PasswordHash)
	if admin.PasswordHash == "" {
		compare = a.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword(compare, []byte(r.PostForm.Get("password")))
	if err != nil || passwordErr != nil {
		a.render(w, r, authviews.Login(a.viewData(w, r, "title.login"), a.message(r, "error.login_invalid")), 422)
		return
	}
	token := randomToken()
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback() }() // Commit 後の ErrTxDone は無視する。
	q := a.queries.WithTx(tx)
	err = q.DeleteExpiredSessions(r.Context())
	if err == nil {
		err = q.CreateSession(r.Context(), store.CreateSessionParams{TokenHash: tokenHash(token), AdminID: admin.ID})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.mu.Lock()
	delete(a.attempts, email)
	a.mu.Unlock()
	a.cookie(w, "session", token, 12*3600)
	a.cookie(w, "csrf", randomToken(), 0)
	redirect(w, r, "/")
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("session")
	if err := a.queries.DeleteSession(r.Context(), tokenHash(cookie.Value)); err != nil {
		a.fail(w, r, err)
		return
	}
	a.cookie(w, "session", "", -1)
	a.cookie(w, "csrf", "", -1)
	redirect(w, r, "/auth/sign_in")
}
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	count, err := a.queries.CountProducts(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, dashboardviews.Index(a.viewData(w, r, "title.dashboard"), count), 200)
}
func (a *App) faq(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, faqviews.Index(a.viewData(w, r, "title.faq")), 200)
}
