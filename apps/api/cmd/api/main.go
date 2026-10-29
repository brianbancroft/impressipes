package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/brianbancroft/impressipies/apps/api/internal/db"
	"github.com/brianbancroft/impressipies/apps/api/internal/server"
	"github.com/brianbancroft/impressipies/apps/api/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Migrations run on boot. Without DATABASE_URL the API still starts on its
	// in-memory store, which is what local development and the tests use.
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pool, err := db.Open(context.Background(), url)
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		defer pool.Close()
		log.Print("database migrated")
	}

	api := server.New()
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api))
	mux.Handle("GET /healthz", api)
	mux.Handle("/", web.Handler())

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
