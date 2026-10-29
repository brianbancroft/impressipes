// Command seed migrates the database and loads the local-development starter
// data (shopping lists and their categories). It is idempotent: the seed lists
// have fixed ids, and if the first is already present nothing is written.
package main

import (
	"context"
	"log"
	"os"

	"github.com/brianbancroft/impressipies/apps/api/internal/db"
)

type category struct{ name, colour, icon string }

type list struct {
	id, name, emoji, colour string
	categories              []category
}

var groceryCategories = []category{
	{"Produce", "#4C9A4A", "🥬"},
	{"Dairy & eggs", "#E8B84A", "🥛"},
	{"Meat & fish", "#C9594B", "🥩"},
	{"Bakery", "#C98A4B", "🍞"},
	{"Pantry", "#8A6B4E", "🥫"},
	{"Frozen", "#4B8FC9", "🧊"},
	{"Household", "#7A7F8C", "🧻"},
}

// seedLists' ids are fixed so a re-run can tell the seed is already loaded.
var seedLists = []list{
	{"5eed0000-0000-4000-8000-000000000001", "Groceries", "🛒", "#4C9A4A", groceryCategories},
	{"5eed0000-0000-4000-8000-000000000002", "Costco", "📦", "#C9594B", groceryCategories[:6]},
	{"5eed0000-0000-4000-8000-000000000003", "Hardware", "🔧", "#4B8FC9", []category{
		{"Tools", "#7A7F8C", "🔨"},
		{"Fasteners", "#8A6B4E", "🔩"},
		{"Paint", "#C98A4B", "🎨"},
		{"Garden", "#4C9A4A", "🌱"},
	}},
}

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("seed: DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url) // also applies pending migrations
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	defer pool.Close()

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lists WHERE id = $1)`, seedLists[0].id).Scan(&exists); err != nil {
		log.Fatalf("seed: %v", err)
	}
	if exists {
		log.Print("seed: already loaded, skipping")
		return
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	defer tx.Rollback(ctx)
	for i, l := range seedLists {
		if _, err := tx.Exec(ctx,
			`INSERT INTO lists (id, name, emoji, colour, sort_order) VALUES ($1, $2, $3, $4, $5)`,
			l.id, l.name, l.emoji, l.colour, i); err != nil {
			log.Fatalf("seed: list %q: %v", l.name, err)
		}
		for j, c := range l.categories {
			if _, err := tx.Exec(ctx,
				`INSERT INTO list_categories (list_id, name, colour, icon, sort_order) VALUES ($1, $2, $3, $4, $5)`,
				l.id, c.name, c.colour, c.icon, j); err != nil {
				log.Fatalf("seed: category %q: %v", c.name, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seed: loaded %d lists", len(seedLists))
}
