package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go-todo/internal/store"
	adminviews "go-todo/internal/views/admins"
	"go-todo/internal/views/components"

	"golang.org/x/crypto/bcrypt"
)

func (a *App) admins(w http.ResponseWriter, r *http.Request) {
	n, offset := pagination(r)
	p := adminviews.IndexData{Pagination: components.Pagination{Query: r.URL.Query().Get("q"), Page: n, Sort: r.URL.Query().Get("sort")}}
	rows, err := a.queries.ListAdmins(r.Context(), store.ListAdminsParams{Search: p.Query, Ascending: p.Sort == "asc", PageOffset: offset})
	if err != nil {
		a.fail(w, err)
		return
	}
	for _, row := range rows {
		p.Admins = append(p.Admins, adminModel(row))
	}
	if len(p.Admins) > 30 {
		p.Admins = p.Admins[:30]
		p.Next = n + 1
	}
	if n > 1 {
		p.Previous = n - 1
	}
	a.render(w, r, adminviews.Index(a.viewData(w, r, "管理者一覧"), p), 200)
}
func (a *App) findAdmin(w http.ResponseWriter, r *http.Request) (Admin, bool) {
	n, err := id(r)
	if err != nil {
		http.NotFound(w, r)
		return Admin{}, false
	}
	row, err := a.queries.FindAdmin(r.Context(), n)
	admin := adminModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return admin, false
	}
	if err != nil {
		a.fail(w, err)
		return admin, false
	}
	return admin, true
}
func (a *App) adminShow(w http.ResponseWriter, r *http.Request) {
	target, ok := a.findAdmin(w, r)
	if !ok {
		return
	}
	a.render(w, r, adminviews.Show(a.viewData(w, r, "管理者詳細"), target), 200)
}
func (a *App) adminForm(w http.ResponseWriter, r *http.Request) {
	target := Admin{Role: "operator"}
	if r.PathValue("id") != "" {
		var ok bool
		target, ok = a.findAdmin(w, r)
		if !ok {
			return
		}
	}
	a.render(w, r, adminviews.Form(a.viewData(w, r, "管理者入力"), target, ""), 200)
}
func (a *App) saveAdmin(w http.ResponseWriter, r *http.Request) {
	target := Admin{Role: "operator"}
	current := r.Context().Value(adminContextKey{}).(*Admin)
	if r.PathValue("id") != "" {
		var ok bool
		target, ok = a.findAdmin(w, r)
		if !ok {
			return
		}
	}
	target.Email = strings.ToLower(strings.TrimSpace(r.PostForm.Get("email")))
	// New accounts are operators. Existing admins cannot change their own role.
	if target.ID != 0 && target.ID != current.ID {
		target.Role = r.PostForm.Get("role")
	}
	password := r.PostForm.Get("password")
	err := validateEmail(target.Email)
	if err == nil && target.Role != "operator" && target.Role != "super_admin" {
		err = errors.New("権限が不正です。")
	}
	if err == nil && (target.ID == 0 || password != "") {
		err = validatePassword(password)
		if err == nil && password != r.PostForm.Get("password_confirmation") {
			err = errors.New("確認用パスワードが一致しません。")
		}
	}
	if err == nil {
		var hash []byte
		if password != "" {
			hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		}
		if err == nil {
			if target.ID == 0 {
				target.ID, err = a.queries.CreateAdmin(r.Context(), store.CreateAdminParams{Email: target.Email, PasswordHash: string(hash)})
			} else {
				// Revoke existing sessions when credentials or permissions change.
				tx, txErr := a.db.BeginTx(r.Context(), nil)
				if txErr != nil {
					a.fail(w, txErr)
					return
				}
				defer func() { _ = tx.Rollback() }() // Commit 後の ErrTxDone は無視する。
				q := a.queries.WithTx(tx)
				err = q.UpdateAdmin(r.Context(), store.UpdateAdminParams{ID: target.ID, Email: target.Email, Role: target.Role, NewPasswordHash: string(hash)})
				if err == nil {
					err = q.DeleteAdminSessions(r.Context(), target.ID)
				}
				if err == nil {
					err = tx.Commit()
				}
			}
		}
		if err != nil {
			var pgErr interface{ SQLState() string }
			if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
				err = errors.New(dbMessage(err))
			} else {
				a.fail(w, err)
				return
			}
		}
	}
	if err != nil {
		a.render(w, r, adminviews.Form(a.viewData(w, r, "管理者入力"), target, err.Error()), 422)
		return
	}
	redirect(w, r, fmt.Sprintf("/console/admins/%d", target.ID))
}
func (a *App) deleteAdmin(w http.ResponseWriter, r *http.Request) {
	target, ok := a.findAdmin(w, r)
	if !ok {
		return
	}
	current := r.Context().Value(adminContextKey{}).(*Admin)
	if target.ID == current.ID {
		http.Error(w, "自分自身は削除できません。", http.StatusUnprocessableEntity)
		return
	}
	if err := a.queries.DiscardAdmin(r.Context(), target.ID); err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/console/admins")
}
