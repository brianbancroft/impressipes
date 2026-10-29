package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brianbancroft/impressipies/apps/api/internal/apple"
	"github.com/brianbancroft/impressipies/apps/api/internal/db"
	"github.com/brianbancroft/impressipies/apps/api/internal/mail"
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

	// SMTP_ADDR (Mailpit in local dev) or RESEND_API_KEY (production) turns on the
	// first-run admin invite email; Resend also needs MAIL_FROM.
	var opts []server.Option
	var mailer mail.Mailer
	from := envOr("MAIL_FROM", "Impressipies <noreply@impressipes.com>")
	if addr := os.Getenv("SMTP_ADDR"); addr != "" {
		mailer = mail.SMTP{Addr: addr, From: from}
	} else if key := os.Getenv("RESEND_API_KEY"); key != "" {
		if os.Getenv("MAIL_FROM") == "" {
			log.Fatal("RESEND_API_KEY is set but MAIL_FROM is not")
		}
		mailer = mail.Resend{APIKey: key, From: from}
	}
	if mailer != nil {
		opts = append(opts, server.WithBootstrapMail(
			mailer,
			envOr("BOOTSTRAP_ADMIN_EMAIL", "admin@impressipes.com"),
			envOr("APP_URL", envOr("PUBLIC_URL", "http://localhost:4200")),
		))
	}

	// APPLE_TEAM_ID lets the emailed sign-in link open the iOS app.
	if team := os.Getenv("APPLE_TEAM_ID"); team != "" {
		opts = append(opts, server.WithAppleTeam(team))
	}

	// Passkeys are bound to the site's domain, so APP_URL (or WEBAUTHN_RP_ID, to share a
	// parent domain) must match where people really sign in.
	appURL := envOr("APP_URL", "http://localhost:4200")
	if u, err := url.Parse(appURL); err == nil && u.Hostname() != "" {
		origins := []string{strings.TrimRight(appURL, "/")}
		for _, o := range strings.Split(os.Getenv("WEBAUTHN_ORIGINS"), ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		opts = append(opts, server.WithPasskeys(envOr("WEBAUTHN_RP_ID", u.Hostname()), origins...))
	}

	// The iOS app signs in with its bundle ID as audience; APPLE_WEB_CLIENT_ID (a Services
	// ID) additionally turns on the web button.
	webClient := os.Getenv("APPLE_WEB_CLIENT_ID")
	opts = append(opts, server.WithApple(apple.New(server.IOSBundleID, webClient), webClient))

	api := server.New(pool, opts...)
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api))
	mux.Handle("GET /healthz", api)
	mux.Handle("/mcp", api) // the MCP endpoint and its OAuth server live at the site root, not under /api
	mux.Handle("/oauth/", api)
	for _, p := range []string{"/authorize", "/token", "/register", "/revoke"} {
		mux.Handle(p, api) // default OAuth paths for clients that skip discovery
	}
	mux.Handle("/.well-known/", api)
	mux.Handle("GET /skill", web.SkillHandler())            // public: the downloadable Claude skill
	mux.Handle("GET /skill.md", web.SkillMarkdownHandler()) // the same skill as text, for assistants that fetch it
	mux.Handle("/", web.Handler())

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
