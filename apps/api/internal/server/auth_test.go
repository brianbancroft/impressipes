package server

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbtest"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

type sentMail struct{ to, subject, body string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (f *fakeMailer) Send(to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMail{to, subject, body})
	return nil
}

func (f *fakeMailer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

// next waits for the n-th email (1-based) and returns its link token and code.
func (f *fakeMailer) next(t *testing.T, n int) (to, tok, code string) {
	t.Helper()
	for i := 0; i < 200 && f.count() < n; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) < n {
		t.Fatalf("expected %d emails, got %d", n, len(f.sent))
	}
	m := f.sent[n-1]
	tok = regexp.MustCompile(`/auth/verify\?t=(\S+)`).FindStringSubmatch(m.body)[1]
	code = regexp.MustCompile(`code in the app: (\d{6})`).FindStringSubmatch(m.body)[1]
	return m.to, tok, code
}

func (f *fakeMailer) none(t *testing.T, n int) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	if got := f.count(); got != n {
		t.Fatalf("expected %d emails, got %d", n, got)
	}
}

func TestEmailAuth(t *testing.T) {
	t.Run("memory", func(t *testing.T) { emailAuth(t, family.NewStore()) })
	t.Run("postgres", func(t *testing.T) { emailAuth(t, family.NewPG(dbtest.Pool(t))) })
}

func emailAuth(t *testing.T, store family.Directory) {
	mailer := &fakeMailer{}
	h := NewWithStore(t.Context(), store, nil, func(o *options) { o.mailer, o.baseURL = mailer, "https://app.test" })

	boot, _ := store.CreateInvite(t.Context(), "")
	rec := do(h, "POST", "/invites/"+boot.Token+"/accept",
		`{"name":"Mum","color":"#e11d48","credentialId":"c1","email":"Mum@Example.com","password":"correct horse"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	mum := sessionOf(t, rec)

	// The address is unconfirmed: it cannot sign in yet, and no sign-in mail goes out for it.
	if rec := do(h, "POST", "/auth/login", `{"email":"mum@example.com","password":"correct horse"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login before confirming: got %d, want 401", rec.Code)
	}
	if rec := do(h, "POST", "/auth/magic", `{"email":"mum@example.com"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("magic before confirming: got %d, want 202", rec.Code)
	}
	mailer.none(t, 1)

	// Confirming with the emailed code adds the address and signs in.
	to, _, code := mailer.next(t, 1)
	if to != "mum@example.com" {
		t.Fatalf("confirmation sent to %q", to)
	}
	if rec := do(h, "POST", "/auth/verify", `{"email":"mum@example.com","code":"000000"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code: got %d, want 401", rec.Code)
	}
	if rec := do(h, "POST", "/auth/verify", `{"email":"mum@example.com","code":"`+code+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body)
	} else if !strings.Contains(rec.Body.String(), `"email":"mum@example.com"`) || !strings.Contains(rec.Body.String(), `"hasPassword":true`) {
		t.Fatalf("member after confirm: %s", rec.Body)
	}
	if rec := do(h, "POST", "/auth/verify", `{"email":"mum@example.com","code":"`+code+`"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: got %d, want 401", rec.Code)
	}

	// Password sign-in.
	if rec := do(h, "POST", "/auth/login", `{"email":"MUM@example.com","password":"correct horse"}`); rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	} else if c := sessionOf(t, rec); do(h, "GET", "/me", "", c).Code != http.StatusOK {
		t.Fatal("session from login does not work")
	}
	if rec := do(h, "POST", "/auth/login", `{"email":"mum@example.com","password":"wrong password"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", rec.Code)
	}
	if rec := do(h, "POST", "/auth/login", `{"email":"nobody@example.com","password":"wrong password"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown email: got %d, want 401", rec.Code)
	}

	// Magic link: single use.
	do(h, "POST", "/auth/magic", `{"email":"mum@example.com"}`)
	_, tok, _ := mailer.next(t, 2)
	rec = do(h, "POST", "/auth/verify", `{"token":"`+tok+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("magic link: %d %s", rec.Code, rec.Body)
	}
	if do(h, "GET", "/me", "", sessionOf(t, rec)).Code != http.StatusOK {
		t.Fatal("session from magic link does not work")
	}
	if rec := do(h, "POST", "/auth/verify", `{"token":"`+tok+`"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused link: got %d, want 401", rec.Code)
	}

	// Magic code: five wrong guesses lock the challenge, even for the right code.
	do(h, "POST", "/auth/magic", `{"email":"mum@example.com"}`)
	_, _, code = mailer.next(t, 3)
	wrong := "111111"
	if code == wrong {
		wrong = "222222"
	}
	for i := 0; i < family.MaxCodeAttempts; i++ {
		do(h, "POST", "/auth/verify", `{"email":"mum@example.com","code":"`+wrong+`"}`)
	}
	if rec := do(h, "POST", "/auth/verify", `{"email":"mum@example.com","code":"`+code+`"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked challenge accepted the right code: %d", rec.Code)
	}

	// Unknown addresses look exactly like known ones and send nothing.
	if rec := do(h, "POST", "/auth/magic", `{"email":"nobody@example.com"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("unknown magic: got %d, want 202", rec.Code)
	}
	if rec := do(h, "POST", "/auth/magic", `{"email":"not an email"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("invalid magic: got %d, want 202", rec.Code)
	}
	mailer.none(t, 3)

	// Passwords: too short is refused, a good one replaces the old.
	if rec := do(h, "PUT", "/me/password", `{"password":"short"}`, mum); rec.Code != http.StatusBadRequest {
		t.Fatalf("short password: got %d, want 400", rec.Code)
	}
	if rec := do(h, "PUT", "/me/password", `{"password":"a brand new one"}`, mum); rec.Code != http.StatusNoContent {
		t.Fatalf("set password: got %d", rec.Code)
	}
	if rec := do(h, "POST", "/auth/login", `{"email":"mum@example.com","password":"correct horse"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password: got %d, want 401", rec.Code)
	}
	if rec := do(h, "POST", "/auth/login", `{"email":"mum@example.com","password":"a brand new one"}`); rec.Code != http.StatusOK {
		t.Fatalf("new password: got %d", rec.Code)
	}

	// A second member cannot take Mum's address, and a changed address needs confirming.
	rec = do(h, "POST", "/invites", "{}", mum)
	dadInvite := tokenOf(t, rec)
	rec = do(h, "POST", "/invites/"+dadInvite+"/accept", `{"name":"Dad","color":"#2563eb","credentialId":"c2","email":"mum@example.com","password":"long enough pw"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate email: got %d, want 409", rec.Code)
	}
	rec = do(h, "POST", "/invites/"+dadInvite+"/accept", `{"name":"Dad","color":"#2563eb","credentialId":"c2","email":"dad@example.com","password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("weak password at accept: got %d, want 400", rec.Code)
	}
	rec = do(h, "POST", "/invites/"+dadInvite+"/accept", `{"name":"Dad","color":"#2563eb","credentialId":"c2"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("new member without email and password: got %d, want 400", rec.Code)
	}
	rec = do(h, "POST", "/invites/"+dadInvite+"/accept", `{"name":"Dad","color":"#2563eb","credentialId":"c2","email":"dad@example.com","password":"long enough pw"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("a refused accept must not burn the invite: %d %s", rec.Code, rec.Body)
	}
	dad := sessionOf(t, rec)
	if rec := do(h, "POST", "/me/email", `{"email":"mum@example.com"}`, dad); rec.Code != http.StatusConflict {
		t.Fatalf("taken email: got %d, want 409", rec.Code)
	}
	if rec := do(h, "POST", "/me/email", `{"email":"nope"}`, dad); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad email: got %d, want 400", rec.Code)
	}
	if rec := do(h, "POST", "/me/email", `{"email":"dad@example.com"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("set email signed out: got %d, want 401", rec.Code)
	}
	if rec := do(h, "POST", "/me/email", `{"email":"dad@example.com"}`, dad); rec.Code != http.StatusAccepted {
		t.Fatalf("set email: got %d", rec.Code)
	}
	if rec := do(h, "GET", "/me", "", dad); strings.Contains(rec.Body.String(), "dad@example.com") {
		t.Fatal("email shown before it was confirmed")
	}
	_, tok, _ = mailer.next(t, 5)
	if rec := do(h, "POST", "/auth/verify", `{"token":"`+tok+`"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "dad@example.com") {
		t.Fatalf("confirm dad: %d %s", rec.Code, rec.Body)
	}

	// Disabled members get neither a link nor a password session.
	var dadM family.Member
	members, _ := store.Members(t.Context())
	for _, m := range members {
		if m.Name == "Dad" {
			dadM = m
		}
	}
	do(h, "POST", "/members/"+dadM.ID+"/disable", "", mum)
	if rec := do(h, "POST", "/auth/magic", `{"email":"dad@example.com"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("disabled magic: got %d", rec.Code)
	}
	mailer.none(t, 5)
}

func TestAuthRateLimits(t *testing.T) {
	h := NewWithStore(t.Context(), family.NewStore(), nil)
	for i := 0; i < 10; i++ {
		if rec := do(h, "POST", "/auth/login", `{"email":"a@example.com","password":"nope nope nope"}`); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, rec.Code)
		}
	}
	if rec := do(h, "POST", "/auth/login", `{"email":"a@example.com","password":"nope nope nope"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the limit: got %d, want 429", rec.Code)
	}
}

func TestAppSiteAssociation(t *testing.T) {
	if rec := do(NewWithStore(t.Context(), family.NewStore(), nil), "GET", "/.well-known/apple-app-site-association", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("without a team: got %d, want 404", rec.Code)
	}
	h := NewWithStore(t.Context(), family.NewStore(), nil, WithAppleTeam("TEAM123"))
	rec := do(h, "GET", "/.well-known/apple-app-site-association", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"TEAM123.ca.brianbancroft.impressipies"`) || !strings.Contains(rec.Body.String(), `"/auth/verify"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
