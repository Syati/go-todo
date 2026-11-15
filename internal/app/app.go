package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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

//go:embed static/*
var assets embed.FS

type App struct {
	db        *sql.DB
	queries   *store.Queries
	secure    bool
	mu        sync.Mutex
	attempts  map[string]attempt
	dummyHash []byte
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
	return &App{db: db, queries: store.New(db), secure: secure, attempts: map[string]attempt{}, dummyHash: dummy}, nil
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
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /up", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := a.db.PingContext(ctx); err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, err := w.Write([]byte("ok\n")); err != nil {
			slog.Warn("health response write failed", "error", err)
		}
	})
	mux.HandleFunc("GET /auth/sign_in", a.login)
	mux.HandleFunc("POST /auth/sign_in", a.login)
	mux.HandleFunc("POST /auth/sign_out", a.authorized(a.logout, false))
	mux.HandleFunc("GET /{$}", a.authorized(a.dashboard, false))
	mux.HandleFunc("GET /faq", a.authorized(a.faq, false))
	mux.HandleFunc("GET /console/products", a.authorized(a.products, false))
	mux.HandleFunc("GET /console/products/new", a.authorized(a.productForm, false))
	mux.HandleFunc("POST /console/products", a.authorized(a.saveProduct, false))
	mux.HandleFunc("GET /console/products/{id}", a.authorized(a.productShow, false))
	mux.HandleFunc("GET /console/products/{id}/edit", a.authorized(a.productForm, false))
	mux.HandleFunc("POST /console/products/{id}", a.authorized(a.saveProduct, false))
	mux.HandleFunc("POST /console/products/{id}/delete", a.authorized(a.deleteProduct, false))
	mux.HandleFunc("GET /console/products/{id}/skus/new", a.authorized(a.skuForm, false))
	mux.HandleFunc("POST /console/products/{id}/skus", a.authorized(a.saveSKU, false))
	mux.HandleFunc("GET /console/products/{id}/skus/{sku}/edit", a.authorized(a.skuForm, false))
	mux.HandleFunc("POST /console/products/{id}/skus/{sku}", a.authorized(a.saveSKU, false))
	mux.HandleFunc("POST /console/products/{id}/skus/{sku}/delete", a.authorized(a.deleteSKU, false))
	mux.HandleFunc("GET /console/admins", a.authorized(a.admins, false))
	mux.HandleFunc("GET /console/admins/new", a.authorized(a.adminForm, true))
	mux.HandleFunc("POST /console/admins", a.authorized(a.saveAdmin, true))
	mux.HandleFunc("GET /console/admins/{id}", a.authorized(a.adminShow, false))
	mux.HandleFunc("GET /console/admins/{id}/edit", a.authorized(a.adminForm, true))
	mux.HandleFunc("POST /console/admins/{id}", a.authorized(a.saveAdmin, true))
	mux.HandleFunc("POST /console/admins/{id}/delete", a.authorized(a.deleteAdmin, true))
	protected := http.NewCrossOriginProtection().Handler(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if r.Method != "GET" && r.Method != "HEAD" {
			cookie, err := r.Cookie("csrf")
			if err != nil || r.ParseForm() != nil || len(cookie.Value) != 64 || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.PostForm.Get("csrf"))) != 1 {
				http.Error(w, "フォームの有効期限が切れました。画面を再読み込みしてください。", http.StatusForbidden)
				return
			}
		}
		protected.ServeHTTP(w, r)
	})
}

type adminContextKey struct{}

func (a *App) authorized(next http.HandlerFunc, super bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/auth/sign_in", http.StatusSeeOther)
			return
		}
		row, err := a.queries.FindSessionAdmin(r.Context(), tokenHash(cookie.Value))
		if errors.Is(err, sql.ErrNoRows) {
			http.Redirect(w, r, "/auth/sign_in", http.StatusSeeOther)
			return
		}
		if err != nil {
			a.fail(w, err)
			return
		}
		admin := adminModel(row)
		if super && admin.Role != "super_admin" {
			http.Error(w, "この操作には特権管理者の権限が必要です。", http.StatusForbidden)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), adminContextKey{}, &admin)))
	}
}

func (a *App) viewData(w http.ResponseWriter, r *http.Request, title string) views.Page {
	p := views.Page{Title: title}
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
		a.fail(w, err)
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
func (a *App) fail(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	http.Error(w, "処理に失敗しました。時間をおいてお試しください。", 500)
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
		a.render(w, r, authviews.Login(a.viewData(w, r, "ログイン"), ""), 200)
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
		a.render(w, r, authviews.Login(a.viewData(w, r, "ログイン"), "ログイン試行が多すぎます。15分後にお試しください。"), 429)
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
		a.fail(w, err)
		return
	}
	compare := []byte(admin.PasswordHash)
	if admin.PasswordHash == "" {
		compare = a.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword(compare, []byte(r.PostForm.Get("password")))
	if err != nil || passwordErr != nil {
		a.render(w, r, authviews.Login(a.viewData(w, r, "ログイン"), "メールアドレスまたはパスワードが正しくありません。"), 422)
		return
	}
	token := randomToken()
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.fail(w, err)
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
		a.fail(w, err)
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
		a.fail(w, err)
		return
	}
	a.cookie(w, "session", "", -1)
	a.cookie(w, "csrf", "", -1)
	redirect(w, r, "/auth/sign_in")
}
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	count, err := a.queries.CountProducts(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, r, dashboardviews.Index(a.viewData(w, r, "Dashboard"), count), 200)
}
func (a *App) faq(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, faqviews.Index(a.viewData(w, r, "よくある質問")), 200)
}
