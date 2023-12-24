package main

import (
	"fmt"
	"github.com/jesseops/coppermind/internal/pkg/routes"
	"github.com/jesseops/coppermind/internal/pkg/services"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	container := services.NewContainer()

	// Add log if err returned during shutdown
	defer func() {
		if err := container.Shutdown(); err != nil {
			container.WebServer.Logger.Fatal(err)
		}
	}()

	// Setup routes
	routes.SetupRoutes(container)

	go func() {
		srv := http.Server{
			Addr: fmt.Sprintf("%s:%d",
				container.Config.HTTP.Host,
				container.Config.HTTP.Port),
			Handler: container.WebServer,
		}
		log.Fatal(srv.ListenAndServe())
	}()
	log.Println(fmt.Sprintf("Server is ready to handle requests at http://%s:%d",
		container.Config.HTTP.Host, container.Config.HTTP.Port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	signal.Notify(quit, os.Kill)
	<-quit
}
