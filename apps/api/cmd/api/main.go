package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brianbancroft/impressipies/apps/api/internal/db"
	"github.com/brianbancroft/impressipies/apps/api/internal/server"
	"github.com/brianbancroft/impressipies/apps/api/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// With DATABASE_URL the API migrates on boot and persists to Postgres. Without
	// it the API runs on its in-memory directory, as local development and tests do.
	var pool *pgxpool.Pool
	if url := os.Getenv("DATABASE_URL"); url != "" {
		var err error
		if pool, err = db.Open(context.Background(), url); err != nil {
			log.Fatalf("database: %v", err)
		}
		defer pool.Close()
		log.Print("database migrated")
	}

	api := server.New(pool)
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api))
	mux.Handle("GET /healthz", api)
	mux.Handle("/", web.Handler())

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
