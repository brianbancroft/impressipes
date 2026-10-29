package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const callback = "https://claude.ai/api/mcp/auth_callback"

type oauthFlow struct {
	*mcpEnv
	hc       *http.Client
	clientID string
	verifier string
}

func (f *oauthFlow) post(path string, form url.Values, cookie bool) (*http.Response, string) {
	f.t.Helper()
	req, _ := http.NewRequest("POST", f.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie {
		req.AddCookie(f.cookie)
	}
	res, err := f.hc.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func newOAuthFlow(t *testing.T) *oauthFlow {
	e := newMCPEnv(t)
	f := &oauthFlow{mcpEnv: e, verifier: "a-long-random-verifier-0123456789-abcdefghijklmnop"}
	f.hc = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := f.hc.Post(e.srv.URL+"/oauth/register", "application/json",
		strings.NewReader(`{"client_name":"Claude","redirect_uris":["`+callback+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		ClientID string `json:"client_id"`
	}
	json.NewDecoder(res.Body).Decode(&reg)
	res.Body.Close()
	if res.StatusCode != 201 || reg.ClientID == "" {
		t.Fatalf("register: %d", res.StatusCode)
	}
	f.clientID = reg.ClientID
	return f
}

func (f *oauthFlow) challenge() string {
	sum := sha256.Sum256([]byte(f.verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (f *oauthFlow) authForm(decision string) url.Values {
	return url.Values{
		"client_id": {f.clientID}, "redirect_uri": {callback}, "response_type": {"code"}, "state": {"xyz"},
		"code_challenge": {f.challenge()}, "code_challenge_method": {"S256"}, "decision": {decision},
	}
}

// authorize approves the consent form and returns the code from the redirect.
func (f *oauthFlow) authorize() string {
	f.t.Helper()
	res, _ := f.post("/oauth/authorize", f.authForm("allow"), true)
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != http.StatusFound || loc.Query().Get("code") == "" || loc.Query().Get("state") != "xyz" {
		f.t.Fatalf("authorize: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	return loc.Query().Get("code")
}

func (f *oauthFlow) token(form url.Values) (int, map[string]any) {
	f.t.Helper()
	res, body := f.post("/oauth/token", form, false)
	out := map[string]any{}
	json.Unmarshal([]byte(body), &out)
	return res.StatusCode, out
}

func (f *oauthFlow) exchange(code string) (int, map[string]any) {
	return f.token(url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {f.clientID},
		"redirect_uri": {callback}, "code_verifier": {f.verifier},
	})
}

func TestOAuthDiscovery(t *testing.T) {
	e := newMCPEnv(t)
	for path, key := range map[string]string{
		"/.well-known/oauth-protected-resource":       "authorization_servers",
		"/.well-known/oauth-authorization-server":     "registration_endpoint",
		"/.well-known/oauth-authorization-server/mcp": "registration_endpoint",
		"/.well-known/openid-configuration":           "registration_endpoint",
		"/.well-known/openid-configuration/mcp":       "registration_endpoint",
	} {
		res, err := http.Get(e.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		json.NewDecoder(res.Body).Decode(&m)
		res.Body.Close()
		if res.StatusCode != 200 || m[key] == nil {
			t.Errorf("%s: %d %v", path, res.StatusCode, m)
		}
	}
	res, _ := http.Post(e.srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if !strings.Contains(res.Header.Get("WWW-Authenticate"), "resource_metadata=") {
		t.Errorf("401 should point at the resource metadata: %q", res.Header.Get("WWW-Authenticate"))
	}
}

func TestOAuthDefaultPaths(t *testing.T) {
	e := newMCPEnv(t)
	res, err := http.Post(e.srv.URL+"/register", "application/json", strings.NewReader(`{"client_name":"x","redirect_uris":["https://example.com/cb"]}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Errorf("/register: %d", res.StatusCode)
	}
	res, err = http.Get(e.srv.URL + "/authorize?client_id=nope")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("/authorize should reach the consent handler: %d", res.StatusCode)
	}
}

func TestOAuthRegisterRejectsBadRedirects(t *testing.T) {
	e := newMCPEnv(t)
	for _, uri := range []string{"http://evil.example/cb", "javascript:alert(1)", "https://x.example/cb#frag", ""} {
		res, _ := http.Post(e.srv.URL+"/oauth/register", "application/json",
			strings.NewReader(`{"client_name":"x","redirect_uris":["`+uri+`"]}`))
		if res.StatusCode != 400 {
			t.Errorf("redirect %q: got %d, want 400", uri, res.StatusCode)
		}
	}
	for _, uri := range []string{"http://localhost:6274/cb", "http://127.0.0.1:9/cb", "claude://callback"} {
		res, _ := http.Post(e.srv.URL+"/oauth/register", "application/json",
			strings.NewReader(`{"client_name":"x","redirect_uris":["`+uri+`"]}`))
		if res.StatusCode != 201 {
			t.Errorf("redirect %q: got %d, want 201", uri, res.StatusCode)
		}
	}
}

func TestOAuthFullFlow(t *testing.T) {
	f := newOAuthFlow(t)

	// Not signed in: a page asking to sign in, not a consent form.
	q := f.authForm("")
	q.Del("decision")
	res, err := f.hc.Get(f.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), `name="decision"`) || !strings.Contains(string(body), "Sign in") {
		t.Errorf("signed-out visitor got %s", body)
	}
	// Signed in: the consent form names the client and the member.
	req, _ := http.NewRequest("GET", f.srv.URL+"/oauth/authorize?"+q.Encode(), nil)
	req.AddCookie(f.cookie)
	res, _ = f.hc.Do(req)
	body, _ = io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Connect Claude?") || !strings.Contains(string(body), "Mum") ||
		res.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("consent page: %s", body)
	}

	// Without a session the approval is refused.
	if res, _ := f.post("/oauth/authorize", f.authForm("allow"), false); res.StatusCode != 401 {
		t.Errorf("approve without session: %d", res.StatusCode)
	}

	code := f.authorize()
	// A wrong verifier burns the code.
	bad := f.verifier
	f.verifier = "not-the-verifier-not-the-verifier-not-the-verifier"
	if status, _ := f.exchange(code); status != 400 {
		t.Errorf("wrong verifier: %d", status)
	}
	f.verifier = bad
	if status, _ := f.exchange(code); status != 400 {
		t.Errorf("a used code must not work again: %d", status)
	}

	code = f.authorize()
	status, tok := f.exchange(code)
	if status != 200 || tok["token_type"] != "Bearer" {
		t.Fatalf("exchange: %d %v", status, tok)
	}
	access, refresh := tok["access_token"].(string), tok["refresh_token"].(string)
	if status, _ := f.exchange(code); status != 400 {
		t.Errorf("replayed code: %d", status)
	}

	// The access token opens /mcp and the change is attributed to the client.
	client := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: f.srv.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{access}},
	}, nil)
	if err != nil {
		t.Fatalf("connect with oauth token: %v", err)
	}
	defer sess.Close()
	if _, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "list_lists", Arguments: obj{}}); err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/lists", `{"name":"Costco","colour":"#C9594B"}`, 201, nil)
	if _, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "add_items", Arguments: obj{"list": "Costco", "items": []obj{{"name": "Milk"}}}}); err != nil {
		t.Fatal(err)
	}
	var feed []obj
	f.call("GET", "/activity?via=mcp", "", 200, &feed)
	if len(feed) == 0 || feed[0]["via"] != "mcp:Claude" {
		t.Errorf("feed = %v", feed)
	}

	// Refresh rotates: the old refresh token dies, the new pair works.
	rf := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {f.clientID}}
	status, tok2 := f.token(rf)
	if status != 200 || tok2["access_token"] == access || tok2["refresh_token"] == refresh {
		t.Fatalf("refresh: %d %v", status, tok2)
	}
	if status, _ := f.token(rf); status != 400 {
		t.Errorf("old refresh token should be dead: %d", status)
	}
	if _, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "list_lists", Arguments: obj{}}); err == nil {
		// The old access token was replaced by the rotation.
		t.Error("old access token should stop working after rotation")
	}

	// Revoking the new access token ends the grant.
	f.post("/oauth/revoke", url.Values{"token": {tok2["access_token"].(string)}, "client_id": {f.clientID}}, false)
	req2, _ := http.NewRequest("POST", f.srv.URL+"/mcp", strings.NewReader(`{}`))
	req2.Header.Set("Authorization", "Bearer "+tok2["access_token"].(string))
	if res, _ := http.DefaultClient.Do(req2); res.StatusCode != 401 {
		t.Errorf("revoked token: %d", res.StatusCode)
	}
	if status, _ := f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok2["refresh_token"].(string)}, "client_id": {f.clientID}}); status != 400 {
		t.Errorf("revoked grant must not refresh: %d", status)
	}
}

func TestOAuthDenyAndBadRequests(t *testing.T) {
	f := newOAuthFlow(t)
	res, _ := f.post("/oauth/authorize", f.authForm("deny"), true)
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != 302 || loc.Query().Get("error") != "access_denied" || loc.Query().Get("code") != "" {
		t.Errorf("deny: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	// A redirect address the client did not register is shown, never followed.
	form := f.authForm("allow")
	form.Set("redirect_uri", "https://evil.example/cb")
	if res, _ := f.post("/oauth/authorize", form, true); res.StatusCode != 400 || res.Header.Get("Location") != "" {
		t.Errorf("unregistered redirect: %d", res.StatusCode)
	}
	// PKCE is mandatory.
	form = f.authForm("allow")
	form.Del("code_challenge")
	if res, _ := f.post("/oauth/authorize", form, true); res.StatusCode != 400 {
		t.Errorf("missing challenge: %d", res.StatusCode)
	}
	// Another client's code is not accepted.
	code := f.authorize()
	status, _ := f.token(url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {"imp-someone-else"},
		"redirect_uri": {callback}, "code_verifier": {f.verifier},
	})
	if status != 400 {
		t.Errorf("wrong client: %d", status)
	}
	if status, _ := f.token(url.Values{"grant_type": {"password"}}); status != 400 {
		t.Errorf("unsupported grant: %d", status)
	}
}
