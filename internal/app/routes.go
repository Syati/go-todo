package app

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"go-todo/assets"

	"github.com/labstack/echo/v5"
)

func (a *App) routes() *echo.Echo {
	e := echo.New()
	e.Use(a.localize)
	e.Use(echo.WrapMiddleware(http.NewCrossOriginProtection().Handler))
	e.Use(a.securityHeaders)

	static, _ := fs.Sub(assets.Files, "static")
	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(static)))))
	e.GET("/up", echo.WrapHandler(http.HandlerFunc(a.health)))

	e.GET("/auth/sign_in", echoHandler(a.login))
	e.POST("/auth/sign_in", echoHandler(a.login))
	authenticated := e.Group("")
	authenticated.Use(a.requireAdmin(false))
	authenticated.POST("/auth/sign_out", echoHandler(a.logout))
	authenticated.GET("/", echoHandler(a.dashboard))
	authenticated.GET("/faq", echoHandler(a.faq))

	console := e.Group("/console")
	console.Use(a.requireAdmin(false))
	products := console.Group("/products")
	products.GET("", echoHandler(a.products))
	products.GET("/new", echoHandler(a.productForm))
	products.POST("", echoHandler(a.saveProduct))
	products.GET("/:id", echoHandler(a.productShow))
	products.GET("/:id/edit", echoHandler(a.productForm))
	products.POST("/:id", echoHandler(a.saveProduct))
	products.POST("/:id/delete", echoHandler(a.deleteProduct))
	products.GET("/:id/skus/new", echoHandler(a.skuForm))
	products.POST("/:id/skus", echoHandler(a.saveSKU))
	products.GET("/:id/skus/:sku/edit", echoHandler(a.skuForm))
	products.POST("/:id/skus/:sku", echoHandler(a.saveSKU))
	products.POST("/:id/skus/:sku/delete", echoHandler(a.deleteSKU))

	admins := console.Group("/admins")
	admins.GET("", echoHandler(a.admins))
	admins.GET("/:id", echoHandler(a.adminShow))
	superAdmins := admins.Group("")
	superAdmins.Use(a.requireAdmin(true))
	superAdmins.GET("/new", echoHandler(a.adminForm))
	superAdmins.POST("", echoHandler(a.saveAdmin))
	superAdmins.GET("/:id/edit", echoHandler(a.adminForm))
	superAdmins.POST("/:id", echoHandler(a.saveAdmin))
	superAdmins.POST("/:id/delete", echoHandler(a.deleteAdmin))

	return e
}

func echoHandler(handler http.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		r := c.Request()
		r.SetPathValue("id", c.Param("id"))
		r.SetPathValue("sku", c.Param("sku"))
		handler(c.Response(), r)
		return nil
	}
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if _, err := w.Write([]byte("ok\n")); err != nil {
		slog.Warn("health response write failed", "error", err)
	}
}
