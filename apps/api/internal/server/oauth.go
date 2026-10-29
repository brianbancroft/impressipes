package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

// The OAuth 2.1 authorization server for MCP clients that cannot hold a personal
// token (claude.ai on the web and phone). Public clients only: dynamic client
// registration (RFC 7591), authorization code with PKCE S256, rotating refresh tokens.
// Consent rides on the member's existing web session.

const (
	oauthAccessPrefix  = "impo_"
	oauthRefreshPrefix = "impr_"
	codeTTL            = 5 * time.Minute
	accessTTL          = time.Hour
	refreshTTL         = 90 * 24 * time.Hour
)

// mountOAuth registers the discovery, registration, authorize and token endpoints.
func (s *server) mountOAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.protectedResource)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", s.protectedResource)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.authServerMetadata)
	mux.HandleFunc("POST /oauth/register", s.oauthRegister)
	mux.HandleFunc("GET /oauth/authorize", s.oauthAuthorize)
	mux.HandleFunc("POST /oauth/authorize", s.oauthDecide)
	mux.HandleFunc("POST /oauth/token", s.oauthToken)
	mux.HandleFunc("POST /oauth/revoke", s.oauthRevoke)
}

// baseURL is the public origin: PUBLIC_URL when set, otherwise what the proxy forwarded.
func baseURL(r *http.Request) string {
	if v := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"); v != "" {
		return v
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *server) protectedResource(w http.ResponseWriter, r *http.Request) {
	b := baseURL(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 b + "/mcp",
		"authorization_servers":    []string{b},
		"bearer_methods_supported": []string{"header"},
	})
}

func (s *server) authServerMetadata(w http.ResponseWriter, r *http.Request) {
	b := baseURL(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                b,
		"authorization_endpoint":                b + "/oauth/authorize",
		"token_endpoint":                        b + "/oauth/token",
		"registration_endpoint":                 b + "/oauth/register",
		"revocation_endpoint":                   b + "/oauth/revoke",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	})
}

func randString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---- dynamic client registration ----

var customScheme = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

// validRedirect accepts https URLs, loopback http URLs (desktop and CLI clients) and
// custom app schemes, and refuses fragments and script-bearing schemes.
func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || raw == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Host != ""
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	case "javascript", "data", "file", "vbscript", "blob", "about":
		return false
	}
	return customScheme.MatchString(u.Scheme)
}

func (s *server) oauthRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
		oauthJSONError(w, http.StatusBadRequest, "invalid_client_metadata", "body must be JSON")
		return
	}
	if len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > 8 {
		oauthJSONError(w, http.StatusBadRequest, "invalid_redirect_uri", "give between 1 and 8 redirect_uris")
		return
	}
	for _, u := range in.RedirectURIs {
		if !validRedirect(u) {
			oauthJSONError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri must be https, a loopback http URL or an app scheme: "+u)
			return
		}
	}
	name := strings.TrimSpace(in.ClientName)
	if name == "" {
		name = "An AI client"
	}
	if len(name) > 80 {
		name = name[:80]
	}
	id := "imp-" + randString(18)
	if _, err := s.q.CreateOAuthClient(r.Context(), dbgen.CreateOAuthClientParams{ID: id, Name: name, RedirectUris: in.RedirectURIs}); err != nil {
		oauthJSONError(w, http.StatusInternalServerError, "server_error", "could not register")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  id,
		"client_name":                name,
		"redirect_uris":              in.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

func oauthJSONError(w http.ResponseWriter, code int, kind, desc string) {
	writeJSON(w, code, map[string]string{"error": kind, "error_description": desc})
}

// ---- authorize ----

// authRequest is a validated authorization request.
type authRequest struct {
	Client      dbgen.OauthClient
	RedirectURI string
	Challenge   string
	State       string
}

// parseAuthRequest validates the parameters. A problem here is shown to the person,
// never redirected, because the redirect target is not yet trusted.
func (s *server) parseAuthRequest(ctx context.Context, v url.Values) (*authRequest, string) {
	client, err := s.q.GetOAuthClient(ctx, v.Get("client_id"))
	if err != nil {
		return nil, "This app is not registered with Impressipes."
	}
	red := v.Get("redirect_uri")
	ok := false
	for _, u := range client.RedirectUris {
		if u == red {
			ok = true
		}
	}
	if red == "" && len(client.RedirectUris) == 1 {
		red, ok = client.RedirectUris[0], true
	}
	if !ok {
		return nil, "The redirect address does not match the one this app registered."
	}
	if v.Get("response_type") != "code" {
		return nil, "Only response_type=code is supported."
	}
	if v.Get("code_challenge") == "" || v.Get("code_challenge_method") != "S256" {
		return nil, "This app must use PKCE (code_challenge_method=S256)."
	}
	return &authRequest{Client: client, RedirectURI: red, Challenge: v.Get("code_challenge"), State: v.Get("state")}, ""
}

func redirectWith(w http.ResponseWriter, r *http.Request, base string, q url.Values) {
	u, _ := url.Parse(base)
	existing := u.Query()
	for k, vs := range q {
		existing[k] = vs
	}
	u.RawQuery = existing.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Impressipes</title>
<style>
body{font:16px/1.5 system-ui,sans-serif;background:#fff8ec;color:#2b2118;margin:0;display:grid;place-items:center;min-height:100vh}
main{background:#fff;border:2px solid #2b2118;border-radius:18px;padding:28px;max-width:420px;margin:16px;box-shadow:5px 5px 0 #2b2118}
h1{font-size:1.3rem;margin:0 0 8px}p{margin:8px 0}.who{opacity:.7;font-size:.9rem}
.row{display:flex;gap:10px;margin-top:20px}
button,a.btn{flex:1;font:inherit;font-weight:600;padding:10px 14px;border-radius:12px;border:2px solid #2b2118;background:#fff;color:#2b2118;cursor:pointer;text-align:center;text-decoration:none}
button.go{background:#f5a623}
</style></head><body><main>
{{if eq .Kind "error"}}
<h1>Can't connect</h1><p>{{.Message}}</p>
{{else if eq .Kind "signin"}}
<h1>Sign in first</h1>
<p>Open <a href="{{.Base}}">{{.Base}}</a> in this browser and sign in to Impressipes, then come back and reload this page.</p>
{{else}}
<h1>Connect {{.Client}}?</h1>
<p><strong>{{.Client}}</strong> will be able to read and change your recipes and shopping lists, as <strong>{{.Member}}</strong>.</p>
<p class="who">Everything it changes shows up in the activity feed and can be undone. It will send you back to {{.Host}}.</p>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.Redirect}}">
<input type="hidden" name="code_challenge" value="{{.Challenge}}">
<input type="hidden" name="code_challenge_method" value="S256">
<input type="hidden" name="response_type" value="code">
<input type="hidden" name="state" value="{{.State}}">
<div class="row"><button name="decision" value="deny" type="submit">Cancel</button><button class="go" name="decision" value="allow" type="submit">Allow</button></div>
</form>
{{end}}
</main></body></html>`))

type page struct {
	Kind, Message, Base, Client, Member, Host, ClientID, Redirect, Challenge, State string
}

func renderPage(w http.ResponseWriter, code int, p page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
	w.WriteHeader(code)
	pageTmpl.Execute(w, p)
}

func (s *server) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	req, msg := s.parseAuthRequest(r.Context(), r.URL.Query())
	if req == nil {
		renderPage(w, http.StatusBadRequest, page{Kind: "error", Message: msg})
		return
	}
	m, fail := s.current(r.Context())
	if fail != nil {
		renderPage(w, http.StatusOK, page{Kind: "signin", Base: baseURL(r)})
		return
	}
	host := req.RedirectURI
	if u, err := url.Parse(req.RedirectURI); err == nil && u.Host != "" {
		host = u.Host
	}
	renderPage(w, http.StatusOK, page{
		Kind: "consent", Client: req.Client.Name, Member: m.Name, Host: host,
		ClientID: req.Client.ID, Redirect: req.RedirectURI, Challenge: req.Challenge, State: req.State,
	})
}

// oauthDecide handles the consent form. The session cookie is SameSite=Lax, so a
// form posted from another site arrives without it and is refused.
func (s *server) oauthDecide(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, http.StatusBadRequest, page{Kind: "error", Message: "That request could not be read."})
		return
	}
	req, msg := s.parseAuthRequest(r.Context(), r.PostForm)
	if req == nil {
		renderPage(w, http.StatusBadRequest, page{Kind: "error", Message: msg})
		return
	}
	m, fail := s.current(r.Context())
	if fail != nil {
		renderPage(w, http.StatusUnauthorized, page{Kind: "signin", Base: baseURL(r)})
		return
	}
	q := url.Values{}
	if req.State != "" {
		q.Set("state", req.State)
	}
	if r.PostForm.Get("decision") != "allow" {
		q.Set("error", "access_denied")
		redirectWith(w, r, req.RedirectURI, q)
		return
	}
	user, _ := uuidx.Parse(m.ID)
	code := randString(32)
	err := s.q.CreateOAuthCode(r.Context(), dbgen.CreateOAuthCodeParams{
		CodeHash: hashToken(code), ClientID: req.Client.ID, UserID: user, RedirectUri: req.RedirectURI,
		CodeChallenge: req.Challenge, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(codeTTL), Valid: true},
	})
	if err != nil {
		renderPage(w, http.StatusInternalServerError, page{Kind: "error", Message: "Something went wrong. Try again."})
		return
	}
	_ = s.q.PurgeOAuthCodes(r.Context())
	q.Set("code", code)
	redirectWith(w, r, req.RedirectURI, q)
}

// ---- token ----

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func newTokens() (access, refresh string) {
	return oauthAccessPrefix + randString(32), oauthRefreshPrefix + randString(32)
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (s *server) oauthToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		oauthJSONError(w, http.StatusBadRequest, "invalid_request", "form-encoded body expected")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.exchangeCode(w, r)
	case "refresh_token":
		s.refreshGrant(w, r)
	default:
		oauthJSONError(w, http.StatusBadRequest, "unsupported_grant_type", "use authorization_code or refresh_token")
	}
}

func (s *server) exchangeCode(w http.ResponseWriter, r *http.Request) {
	ctx, f := r.Context(), r.PostForm
	code, err := s.q.ConsumeOAuthCode(ctx, hashToken(f.Get("code")))
	if err != nil {
		oauthJSONError(w, http.StatusBadRequest, "invalid_grant", "the code is unknown, used or expired")
		return
	}
	sum := sha256.Sum256([]byte(f.Get("code_verifier")))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	switch {
	case code.ClientID != f.Get("client_id"):
		oauthJSONError(w, http.StatusBadRequest, "invalid_grant", "the code was issued to another client")
	case code.RedirectUri != f.Get("redirect_uri"):
		oauthJSONError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match")
	case f.Get("code_verifier") == "" || subtle.ConstantTimeCompare([]byte(want), []byte(code.CodeChallenge)) != 1:
		oauthJSONError(w, http.StatusBadRequest, "invalid_grant", "code_verifier does not match the challenge")
	default:
		u, err := s.q.GetUser(ctx, code.UserID)
		if err != nil || u.DisabledAt.Valid {
			oauthJSONError(w, http.StatusBadRequest, "invalid_grant", "the member is not active")
			return
		}
		access, refresh := newTokens()
		now := time.Now()
		if _, err := s.q.CreateOAuthGrant(ctx, dbgen.CreateOAuthGrantParams{
			ClientID: code.ClientID, UserID: code.UserID, TokenHash: hashToken(access), ExpiresAt: ts(now.Add(accessTTL)),
			RefreshHash: pgtype.Text{String: hashToken(refresh), Valid: true}, RefreshExpiresAt: ts(now.Add(refreshTTL)),
		}); err != nil {
			oauthJSONError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
			return
		}
		writeJSON(w, http.StatusOK, tokenResponse{access, "Bearer", int(accessTTL.Seconds()), refresh})
	}
}

func (s *server) refreshGrant(w http.ResponseWriter, r *http.Request) {
	ctx, f := r.Context(), r.PostForm
	access, refresh := newTokens()
	now := time.Now()
	err := s.withTx(ctx, func(q *dbgen.Queries) error {
		g, err := q.GetOAuthGrantByRefreshForUpdate(ctx, pgtype.Text{String: hashToken(f.Get("refresh_token")), Valid: true})
		if err != nil {
			return err
		}
		if g.ClientID != f.Get("client_id") {
			return pgx.ErrNoRows
		}
		u, err := q.GetUser(ctx, g.UserID)
		if err != nil {
			return err
		}
		if u.DisabledAt.Valid {
			return pgx.ErrNoRows
		}
		return q.RotateOAuthGrant(ctx, dbgen.RotateOAuthGrantParams{
			ID: g.ID, TokenHash: hashToken(access), ExpiresAt: ts(now.Add(accessTTL)),
			RefreshHash: pgtype.Text{String: hashToken(refresh), Valid: true}, RefreshExpiresAt: ts(now.Add(refreshTTL)),
		})
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		oauthJSONError(w, http.StatusBadRequest, "invalid_grant", "the refresh token is unknown, used or expired")
	case err != nil:
		oauthJSONError(w, http.StatusInternalServerError, "server_error", "could not refresh")
	default:
		writeJSON(w, http.StatusOK, tokenResponse{access, "Bearer", int(accessTTL.Seconds()), refresh})
	}
}

// oauthRevoke implements RFC 7009. It answers 200 whether or not the token was known.
func (s *server) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err == nil {
		_, _ = s.q.RevokeOAuthGrantByToken(r.Context(), dbgen.RevokeOAuthGrantByTokenParams{
			TokenHash: hashToken(r.PostForm.Get("token")), ClientID: r.PostForm.Get("client_id"),
		})
	}
	w.WriteHeader(http.StatusOK)
}

// oauthBearer resolves an OAuth access token to its member and client name.
func (s *server) oauthBearer(ctx context.Context, tok string) (*family.Member, string, bool) {
	row, err := s.q.GetOAuthGrantByAccess(ctx, hashToken(tok))
	if err != nil {
		return nil, "", false
	}
	_ = s.q.TouchOAuthGrant(ctx, row.GrantID)
	status := family.StatusActive
	if row.DisabledAt.Valid {
		status = family.StatusDisabled
	}
	return &family.Member{
		ID: uuidx.String(row.ID), Name: row.DisplayName, Color: row.AvatarColor, Role: row.Role, Status: status,
	}, row.ClientName, true
}
