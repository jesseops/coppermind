package routes

import (
	"github.com/jesseops/coppermind/internal/app/services"
	"github.com/labstack/echo/v4"
	"net/http"
)

func errRoutes(c *services.Container, g *echo.Group) {
	g.GET("/400", func(ctx echo.Context) error {
		return ctx.String(http.StatusBadRequest, "400")
	})
	g.GET("/401", func(ctx echo.Context) error {
		return ctx.String(http.StatusUnauthorized, "401")
	})
	g.GET("/403", func(ctx echo.Context) error {
		return ctx.String(http.StatusForbidden, "403")
	})
	g.GET("/404", func(ctx echo.Context) error {
		return ctx.String(http.StatusNotFound, "404")
	})
	g.GET("/418", func(ctx echo.Context) error {
		return ctx.String(http.StatusTeapot, "418")
	})
	g.GET("/429", func(ctx echo.Context) error {
		return ctx.String(http.StatusTooManyRequests, "429")
	})
	g.GET("/500", func(ctx echo.Context) error {
		return ctx.String(http.StatusInternalServerError, "500")
	})
	g.GET("/503", func(ctx echo.Context) error {
		return ctx.String(http.StatusServiceUnavailable, "503")
	})
	g.GET("/panic", func(ctx echo.Context) error {
		panic("test")
	})
}
