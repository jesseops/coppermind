package routes

import (
	"github.com/jesseops/coppermind/internal/app/services"
	echomw "github.com/labstack/echo/v4/middleware"
)

func SetupRoutes(c *services.Container) {
	group := c.WebServer.Group("")
	group.Use(echomw.Logger())

	// Error routes
	errRoutes(c, group)
}
