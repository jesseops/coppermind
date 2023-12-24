package routes

import (
    "github.com/jesseops/coppermind/internal/app/services"
)

func SetupRoutes(c *services.Container) {
    group := c.WebServer.Group("")

    // Error routes
    errRoutes(c, group)
}

