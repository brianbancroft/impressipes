package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbtest"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

func do(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func tokenOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var v struct{ Token string }
	json.NewDecoder(rec.Body).Decode(&v)
	if v.Token == "" {
		t.Fatalf("no token in %s", rec.Body.String())
	}
	return v.Token
}

func TestHealth(t *testing.T) {
	if rec := do(New(nil), http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}

func TestInviteOnboardingFlow(t *testing.T) {
	t.Run("memory", func(t *testing.T) { inviteOnboardingFlow(t, family.NewStore()) })
	t.Run("postgres", func(t *testing.T) { inviteOnboardingFlow(t, family.NewPG(dbtest.Pool(t))) })
}

func inviteOnboardingFlow(t *testing.T, store family.Directory) {
	h := NewWithStore(t.Context(), store, nil)

	boot, _ := store.CreateInvite(context.Background(), "")
	accept := `{"name":"Mum","color":"#e11d48","credentialId":"cred-1","email":"mum@example.com","password":"long enough pw"}`
	rec := do(h, "POST", "/invites/"+boot.Token+"/accept", accept)
	if rec.Code != http.StatusCreated {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	admin := sessionOf(t, rec)

	if rec := do(h, "POST", "/invites/"+boot.Token+"/accept", accept); rec.Code != http.StatusGone {
		t.Fatalf("reused invite: got %d, want 410", rec.Code)
	}

	rec = do(h, "POST", "/invites", "{}", admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create invite: %d", rec.Code)
	}
	tok := tokenOf(t, rec)
	rec = do(h, "POST", "/invites/"+tok+"/accept", `{"name":"Dad","color":"#2563eb","credentialId":"cred-2","email":"dad@example.com","password":"long enough pw"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("member accept: %d", rec.Code)
	}
	dad := sessionOf(t, rec)
	var dadM family.Member
	json.NewDecoder(rec.Body).Decode(&dadM)
	if dadM.Role != family.RoleMember {
		t.Fatalf("second member role = %q, want member", dadM.Role)
	}
	if rec := do(h, "GET", "/members", "", dad); rec.Code != http.StatusForbidden {
		t.Fatalf("member listing members: got %d, want 403", rec.Code)
	}

	// Lost device: re-invite keeps the member, replaces the passkey.
	rec = do(h, "POST", "/invites", `{"memberId":"`+dadM.ID+`"}`, admin)
	re := tokenOf(t, rec)
	rec = do(h, "GET", "/invites/"+re, "")
	if !strings.Contains(rec.Body.String(), `"reinvite":true`) || !strings.Contains(rec.Body.String(), "Dad") {
		t.Fatalf("reinvite lookup: %s", rec.Body)
	}
	rec = do(h, "POST", "/invites/"+re+"/accept", `{"name":"Dad","color":"#2563eb","credentialId":"cred-3"}`)
	var again family.Member
	json.NewDecoder(rec.Body).Decode(&again)
	if rec.Code != http.StatusCreated || again.ID != dadM.ID {
		t.Fatalf("reinvite accept: %d id=%q want %q", rec.Code, again.ID, dadM.ID)
	}

	// Disabled: session is rejected with 403 and re-invites are refused.
	if rec := do(h, "POST", "/members/"+dadM.ID+"/disable", "", admin); rec.Code != http.StatusNoContent {
		t.Fatalf("disable: %d", rec.Code)
	}
	if rec := do(h, "GET", "/me", "", dad); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Fatalf("disabled /me: got %d", rec.Code)
	}
	if rec := do(h, "POST", "/invites", `{"memberId":"`+dadM.ID+`"}`, admin); rec.Code != http.StatusConflict {
		t.Fatalf("reinvite disabled: got %d, want 409", rec.Code)
	}
}
