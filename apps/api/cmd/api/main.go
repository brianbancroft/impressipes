package main

import (
	"log"
	"net/http"
	"os"

	"github.com/brianbancroft/impressipies/apps/api/internal/server"
	"github.com/brianbancroft/impressipies/apps/api/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	api := server.New()
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api))
	mux.Handle("GET /healthz", api)
	mux.Handle("/", web.Handler())

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
