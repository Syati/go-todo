package app

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go-todo/assets"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func (a *App) routes() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = a.handleHTTPError
	e.Use(middleware.Recover())
	e.Use(a.localize)
	e.Use(middleware.BodyLimit(64 << 10))
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		ContentSecurityPolicy: "default-src 'self'; style-src 'self'; script-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'",
		ReferrerPolicy:        "same-origin",
	}))
	e.Use(echo.WrapMiddleware(http.NewCrossOriginProtection().Handler))
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "form:csrf",
		ContextKey:     "csrf",
		CookieName:     "csrf",
		CookiePath:     "/",
		CookieSecure:   a.secure,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteLaxMode,
		Skipper: func(c *echo.Context) bool {
			path := c.Request().URL.Path
			return path == "/up" || strings.HasPrefix(path, "/static/")
		},
	}))
	static, _ := fs.Sub(assets.Files, "static")
	e.StaticFS("/static", static)
	e.GET("/up", a.health)

	e.GET("/auth/sign_in", a.login)
	e.POST("/auth/sign_in", a.login)
	authenticated := e.Group("")
	authenticated.Use(a.requireAdmin(false))
	authenticated.POST("/auth/sign_out", a.logout)
	authenticated.GET("/", a.dashboard)
	authenticated.GET("/faq", a.faq)

	console := e.Group("/console")
	console.Use(a.requireAdmin(false))
	products := console.Group("/products")
	products.GET("", a.products)
	products.GET("/new", a.productForm)
	products.POST("", a.saveProduct)
	products.GET("/:id", a.productShow)
	products.GET("/:id/edit", a.productForm)
	products.POST("/:id", a.saveProduct)
	products.POST("/:id/delete", a.deleteProduct)
	products.GET("/:id/skus/new", a.skuForm)
	products.POST("/:id/skus", a.saveSKU)
	products.GET("/:id/skus/:sku/edit", a.skuForm)
	products.POST("/:id/skus/:sku", a.saveSKU)
	products.POST("/:id/skus/:sku/delete", a.deleteSKU)

	admins := console.Group("/admins")
	admins.GET("", a.admins)
	admins.GET("/:id", a.adminShow)
	superAdmins := admins.Group("")
	superAdmins.Use(a.requireAdmin(true))
	superAdmins.GET("/new", a.adminForm)
	superAdmins.POST("", a.saveAdmin)
	superAdmins.GET("/:id/edit", a.adminForm)
	superAdmins.POST("/:id", a.saveAdmin)
	superAdmins.POST("/:id/delete", a.deleteAdmin)

	return e
}

func (a *App) health(c *echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		slog.Warn("health check failed", "error", err)
		return c.String(http.StatusServiceUnavailable, "unavailable\n")
	}
	return c.String(http.StatusOK, "ok\n")
}
