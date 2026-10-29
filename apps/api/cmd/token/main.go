// Command token mints a personal MCP token for a family member and prints it once.
// Until the Settings page can do this, it is how Claude Code or Desktop gets in:
//
//	DATABASE_URL=... go run ./cmd/token -member "Mum" -name "Claude Code - laptop"
//
// The name is what the activity feed shows for changes made with the token.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/brianbancroft/impressipies/apps/api/internal/db"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/server"
)

func main() {
	member := flag.String("member", "", "display name of the member the token acts as")
	name := flag.String("name", "", "client name shown in the activity feed, e.g. \"Claude Code - laptop\"")
	flag.Parse()
	url := os.Getenv("DATABASE_URL")
	if url == "" || *member == "" || *name == "" {
		log.Fatal("usage: DATABASE_URL=... token -member <display name> -name <client name>")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	defer pool.Close()
	users, err := dbgen.New(pool).ListUsers(ctx)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	for _, u := range users {
		if strings.EqualFold(u.DisplayName, *member) && !u.DisabledAt.Valid {
			id := fmt.Sprintf("%x-%x-%x-%x-%x", u.ID.Bytes[0:4], u.ID.Bytes[4:6], u.ID.Bytes[6:8], u.ID.Bytes[8:10], u.ID.Bytes[10:16])
			tok, err := server.NewToken(ctx, pool, id, *name)
			if err != nil {
				log.Fatalf("token: %v", err)
			}
			fmt.Println(tok)
			return
		}
	}
	log.Fatalf("token: no active member named %q", *member)
}
