package services

import (
	"github.com/jesseops/coppermind/internal/pkg/config"
	"github.com/labstack/echo/v4"
    "log"
)

type Container struct {
	WebServer *echo.Echo
	Config    *config.Config
}

func NewContainer() *Container {
	c := new(Container)
	c.initConfig()
	c.initWeb()
	return c
}

func (c *Container) initConfig() {
    cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
    c.Config = &cfg
}

func (c *Container) initWeb() {
	c.WebServer = echo.New()
	switch c.Config.App.Env {
	case config.EnvProd:
		c.WebServer.Debug = false
	default:
		c.WebServer.Debug = true
	}
}

func (c *Container) Shutdown() error {
	log.Println("Shutting down...")
	return nil
}
