package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	go func() {
		srv := http.Server{
			Addr: "0.0.0.0:8080",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Received request from %s for %s", r.RemoteAddr, r.URL)
				fmt.Fprintf(w, "Hello, World!")
			}),
		}
		log.Fatal(srv.ListenAndServe())
	}()
	log.Println("Server is ready to handle requests at http://localhost:8080")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	signal.Notify(quit, os.Kill)
	<-quit
	log.Println("Shutting down...")
}
