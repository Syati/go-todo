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

	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

func (a *App) admins(c *echo.Context) error {
	n, offset := pagination(c.QueryParam("page"))
	p := adminviews.IndexData{Pagination: components.Pagination{Query: c.QueryParam("q"), Page: n, Sort: c.QueryParam("sort")}}
	rows, err := a.queries.ListAdmins(c.Request().Context(), store.ListAdminsParams{Search: p.Query, Ascending: p.Sort == "asc", PageOffset: offset})
	if err != nil {
		return err
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
	return a.render(c, adminviews.Index(a.viewData(c, "title.admins"), p), http.StatusOK)
}

func (a *App) findAdmin(c *echo.Context) (Admin, error) {
	n, err := id(c)
	if err != nil {
		return Admin{}, echo.ErrNotFound
	}
	row, err := a.queries.FindAdmin(c.Request().Context(), n)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, echo.ErrNotFound
	}
	if err != nil {
		return Admin{}, err
	}
	return adminModel(row), nil
}

func (a *App) adminShow(c *echo.Context) error {
	target, err := a.findAdmin(c)
	if err != nil {
		return err
	}
	return a.render(c, adminviews.Show(a.viewData(c, "title.admin_detail"), target), http.StatusOK)
}

func (a *App) adminForm(c *echo.Context) error {
	target := Admin{Role: "operator"}
	if c.Param("id") != "" {
		var err error
		target, err = a.findAdmin(c)
		if err != nil {
			return err
		}
	}
	return a.render(c, adminviews.Form(a.viewData(c, "title.admin_form"), target, ""), http.StatusOK)
}

func (a *App) saveAdmin(c *echo.Context) error {
	target := Admin{Role: "operator"}
	current := c.Get(adminContextKey).(*Admin)
	if c.Param("id") != "" {
		var err error
		target, err = a.findAdmin(c)
		if err != nil {
			return err
		}
	}
	target.Email = strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	// New accounts are operators. Existing admins cannot change their own role.
	if target.ID != 0 && target.ID != current.ID {
		target.Role = c.FormValue("role")
	}
	password := c.FormValue("password")
	err := validateEmail(target.Email)
	if err == nil && target.Role != "operator" && target.Role != "super_admin" {
		err = errors.New("権限が不正です。")
	}
	if err == nil && (target.ID == 0 || password != "") {
		err = validatePassword(password)
		if err == nil && password != c.FormValue("password_confirmation") {
			err = errors.New("確認用パスワードが一致しません。")
		}
	}
	if err == nil {
		var hash []byte
		if password != "" {
			hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		}
		if err == nil {
			ctx := c.Request().Context()
			err = a.auditedMutation(c, func(q *store.Queries) error {
				if target.ID == 0 {
					var createErr error
					target.ID, createErr = q.CreateAdmin(ctx, store.CreateAdminParams{Email: target.Email, PasswordHash: string(hash)})
					return createErr
				}
				if err := q.UpdateAdmin(ctx, store.UpdateAdminParams{ID: target.ID, Email: target.Email, Role: target.Role, NewPasswordHash: string(hash)}); err != nil {
					return err
				}
				return q.DeleteAdminSessions(ctx, target.ID)
			})
		}
		if err != nil {
			var pgErr interface{ SQLState() string }
			if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
				err = errors.New(dbMessage(err))
			} else {
				return err
			}
		}
	}
	if err != nil {
		return a.render(c, adminviews.Form(a.viewData(c, "title.admin_form"), target, a.inputError(c, err)), http.StatusUnprocessableEntity)
	}
	return redirect(c, fmt.Sprintf("/console/admins/%d", target.ID))
}

func (a *App) deleteAdmin(c *echo.Context) error {
	target, err := a.findAdmin(c)
	if err != nil {
		return err
	}
	current := c.Get(adminContextKey).(*Admin)
	if target.ID == current.ID {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "error.self_delete")
	}
	if err := a.auditedMutation(c, func(q *store.Queries) error {
		return q.DiscardAdmin(c.Request().Context(), target.ID)
	}); err != nil {
		return err
	}
	return redirect(c, "/console/admins")
}
