package main

import (
	"log"
	"net/http"
	"os"

	"github.com/brianbancroft/impressipies/apps/api/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, server.New()))
}
