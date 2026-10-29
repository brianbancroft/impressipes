package server

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brianbancroft/impressipies/apps/api/internal/apple"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbtest"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

// ---- a software authenticator ----

type authenticator struct {
	key  *ecdsa.PrivateKey
	id   []byte
	user string // user handle, learned at registration
	n    uint32
}

func newAuthenticator(t *testing.T) *authenticator {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	rand.Read(id)
	return &authenticator{key: k, id: id}
}

func cborHead(major byte, n int) []byte {
	switch {
	case n < 24:
		return []byte{major<<5 | byte(n)}
	case n < 256:
		return []byte{major<<5 | 24, byte(n)}
	default:
		return []byte{major<<5 | 25, byte(n >> 8), byte(n)}
	}
}

func cborInt(n int) []byte {
	if n >= 0 {
		return cborHead(0, n)
	}
	return cborHead(1, -1-n)
}
func cborBytes(b []byte) []byte { return append(cborHead(2, len(b)), b...) }
func cborText(s string) []byte  { return append(cborHead(3, len(s)), s...) }

func (a *authenticator) coseKey() []byte {
	x, y := a.key.X.FillBytes(make([]byte, 32)), a.key.Y.FillBytes(make([]byte, 32))
	var b []byte
	b = append(b, cborHead(5, 5)...)
	b = append(b, cborInt(1)...)
	b = append(b, cborInt(2)...)
	b = append(b, cborInt(3)...)
	b = append(b, cborInt(-7)...)
	b = append(b, cborInt(-1)...)
	b = append(b, cborInt(1)...)
	b = append(b, cborInt(-2)...)
	b = append(b, cborBytes(x)...)
	b = append(b, cborInt(-3)...)
	b = append(b, cborBytes(y)...)
	return b
}

func b64s(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func clientData(typ, challenge, origin string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": origin})
	return b
}

// create answers a registration ceremony the way a browser would.
func (a *authenticator) create(t *testing.T, opts map[string]any, rpID, origin string) map[string]any {
	t.Helper()
	pk := opts["publicKey"].(map[string]any)
	a.user = pk["user"].(map[string]any)["id"].(string)
	rp := sha256.Sum256([]byte(rpID))
	authData := append([]byte{}, rp[:]...)
	authData = append(authData, 0x45) // UP | UV | AT
	authData = binary.BigEndian.AppendUint32(authData, 0)
	authData = append(authData, make([]byte, 16)...) // AAGUID
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(a.id)))
	authData = append(authData, a.id...)
	authData = append(authData, a.coseKey()...)

	var att []byte
	att = append(att, cborHead(5, 3)...)
	att = append(att, cborText("fmt")...)
	att = append(att, cborText("none")...)
	att = append(att, cborText("attStmt")...)
	att = append(att, cborHead(5, 0)...)
	att = append(att, cborText("authData")...)
	att = append(att, cborBytes(authData)...)

	return map[string]any{
		"id": b64s(a.id), "rawId": b64s(a.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64s(clientData("webauthn.create", pk["challenge"].(string), origin)),
			"attestationObject": b64s(att),
			"transports":        []string{"internal", "hybrid"},
		},
	}
}

// get answers a sign-in ceremony.
func (a *authenticator) get(t *testing.T, opts map[string]any, rpID, origin string) map[string]any {
	t.Helper()
	pk := opts["publicKey"].(map[string]any)
	rp := sha256.Sum256([]byte(rpID))
	a.n++
	authData := append([]byte{}, rp[:]...)
	authData = append(authData, 0x05) // UP | UV
	authData = binary.BigEndian.AppendUint32(authData, a.n)
	cd := clientData("webauthn.get", pk["challenge"].(string), origin)
	cdHash := sha256.Sum256(cd)
	sum := sha256.Sum256(append(append([]byte{}, authData...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"id": b64s(a.id), "rawId": b64s(a.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": b64s(cd), "authenticatorData": b64s(authData),
			"signature": b64s(sig), "userHandle": a.user,
		},
	}
}

func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type ceremonyResp struct {
	CeremonyID string         `json:"ceremonyId"`
	Options    map[string]any `json:"options"`
}

func TestPasskeys(t *testing.T) {
	t.Run("memory", func(t *testing.T) { passkeyFlow(t, family.NewStore()) })
	t.Run("postgres", func(t *testing.T) { passkeyFlow(t, family.NewPG(dbtest.Pool(t))) })
}

func passkeyFlow(t *testing.T, store family.Directory) {
	const rpID, origin = "app.test", "https://app.test"
	h := NewWithStore(t.Context(), store, nil, WithPasskeys(rpID, origin))

	boot, _ := store.CreateInvite(t.Context(), "")
	rec := do(h, "POST", "/invites/"+boot.Token+"/accept",
		`{"name":"Mum","color":"#e11d48","credentialId":"c1","email":"mum@example.com","password":"correct horse"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	mum := sessionOf(t, rec)

	if rec := do(h, "POST", "/me/passkeys/begin", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous registration: got %d, want 401", rec.Code)
	}

	// Register a passkey.
	phone := newAuthenticator(t)
	rec = do(h, "POST", "/me/passkeys/begin", "", mum)
	if rec.Code != http.StatusOK {
		t.Fatalf("begin registration: %d %s", rec.Code, rec.Body)
	}
	var begin ceremonyResp
	json.NewDecoder(rec.Body).Decode(&begin)
	cred := phone.create(t, begin.Options, rpID, origin)
	rec = do(h, "POST", "/me/passkeys/finish", jsonBody(t, map[string]any{"ceremonyId": begin.CeremonyID, "credential": cred, "name": "iPhone"}), mum)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"name":"iPhone"`) {
		t.Fatalf("finish registration: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/me/passkeys/finish", jsonBody(t, map[string]any{"ceremonyId": begin.CeremonyID, "credential": cred}), mum); rec.Code != http.StatusBadRequest {
		t.Fatalf("replayed ceremony: got %d, want 400", rec.Code)
	}
	if rec := do(h, "GET", "/me/sign-in", "", mum); !strings.Contains(rec.Body.String(), `"name":"iPhone"`) {
		t.Fatalf("sign-in methods: %s", rec.Body)
	}

	// Sign in with it, without saying who is signing in.
	signIn := func(a *authenticator, o string) *httptest.ResponseRecorder {
		rec := do(h, "POST", "/auth/passkey/begin", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("begin sign-in: %d %s", rec.Code, rec.Body)
		}
		var c ceremonyResp
		json.NewDecoder(rec.Body).Decode(&c)
		return do(h, "POST", "/auth/passkey/finish", jsonBody(t, map[string]any{"ceremonyId": c.CeremonyID, "credential": a.get(t, c.Options, rpID, o)}))
	}
	rec = signIn(phone, origin)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Mum"`) {
		t.Fatalf("passkey sign-in: %d %s", rec.Code, rec.Body)
	}
	if do(h, "GET", "/me", "", sessionOf(t, rec)).Code != http.StatusOK {
		t.Fatal("session from passkey does not work")
	}
	if rec := signIn(phone, "https://evil.test"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong origin: got %d, want 401", rec.Code)
	}
	if rec := signIn(newAuthenticator(t), origin); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown passkey: got %d, want 401", rec.Code)
	}

	// Remove it.
	if rec := do(h, "DELETE", "/me/passkeys/"+b64s(phone.id), "", mum); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := signIn(phone, origin); rec.Code != http.StatusUnauthorized {
		t.Fatalf("deleted passkey: got %d, want 401", rec.Code)
	}
}

// ---- Sign in with Apple ----

type appleIssuer struct {
	key *rsa.PrivateKey
}

func newAppleIssuer(t *testing.T) (*appleIssuer, *apple.Verifier) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v := &apple.Verifier{
		Audiences: []string{IOSBundleID},
		Keys: func(context.Context) (map[string]*rsa.PublicKey, error) {
			return map[string]*rsa.PublicKey{"k1": &k.PublicKey}, nil
		},
	}
	return &appleIssuer{k}, v
}

func (a *appleIssuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string { b, _ := json.Marshal(v); return b64s(b) }
	signed := enc(map[string]string{"alg": "RS256", "kid": "k1"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + b64s(sig)
}

func appleClaims(sub, email, nonce string) map[string]any {
	h := sha256.Sum256([]byte(nonce))
	return map[string]any{
		"iss": apple.Issuer, "aud": IOSBundleID, "sub": sub, "email": email, "email_verified": "true",
		"nonce": hex.EncodeToString(h[:]), "exp": time.Now().Add(time.Hour).Unix(),
	}
}

func TestSignInWithApple(t *testing.T) {
	t.Run("memory", func(t *testing.T) { appleFlow(t, family.NewStore()) })
	t.Run("postgres", func(t *testing.T) { appleFlow(t, family.NewPG(dbtest.Pool(t))) })
}

func appleFlow(t *testing.T, store family.Directory) {
	iss, v := newAppleIssuer(t)
	h := NewWithStore(t.Context(), store, nil, WithApple(v, "web.service"))

	boot, _ := store.CreateInvite(t.Context(), "")
	rec := do(h, "POST", "/invites/"+boot.Token+"/accept", `{"name":"Mum","color":"#e11d48","credentialId":"c1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("accept without credentials: %d", rec.Code) // email and password are required to join
	}
	rec = do(h, "POST", "/invites/"+boot.Token+"/accept",
		`{"name":"Mum","color":"#e11d48","credentialId":"c1","email":"mum@example.com","password":"correct horse"}`)
	mum := sessionOf(t, rec)
	// Confirm the address, as the emailed code would.
	ch, err := store.StartVerify(t.Context(), mustMember(t, h, mum), "mum@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CompleteLink(t.Context(), ch.Token); err != nil {
		t.Fatal(err)
	}

	body := func(sub, email, nonce, sent string) string {
		return jsonBody(t, map[string]any{"identityToken": iss.token(t, appleClaims(sub, email, nonce)), "nonce": sent})
	}
	if rec := do(h, "POST", "/auth/apple", body("apple-1", "someone@icloud.com", "n1", "n1")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unlinked Apple ID: got %d, want 401", rec.Code)
	}
	if rec := do(h, "PUT", "/me/apple", body("apple-1", "someone@icloud.com", "n2", "other"), mum); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong nonce: got %d, want 400", rec.Code)
	}
	if rec := do(h, "PUT", "/me/apple", body("apple-1", "someone@icloud.com", "n3", "n3"), mum); rec.Code != http.StatusOK {
		t.Fatalf("link: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/auth/apple", body("apple-1", "someone@icloud.com", "n4", "n4")); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Mum"`) {
		t.Fatalf("sign in: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/me/sign-in", "", mum); !strings.Contains(rec.Body.String(), "someone@icloud.com") {
		t.Fatalf("methods: %s", rec.Body)
	}

	// Someone else's Apple ID cannot be taken over.
	inv, _ := store.CreateInvite(t.Context(), "")
	rec = do(h, "POST", "/invites/"+inv.Token+"/accept",
		`{"name":"Dad","color":"#0284c7","credentialId":"c2","email":"dad@example.com","password":"correct horse 2"}`)
	dad := sessionOf(t, rec)
	if rec := do(h, "PUT", "/me/apple", body("apple-1", "x@icloud.com", "n5", "n5"), dad); rec.Code != http.StatusConflict {
		t.Fatalf("taken Apple ID: got %d, want 409", rec.Code)
	}

	// Unlink, then a verified email that matches a confirmed member links on first use.
	if rec := do(h, "DELETE", "/me/apple", "", mum); rec.Code != http.StatusNoContent {
		t.Fatalf("unlink: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/auth/apple", body("apple-1", "someone@icloud.com", "n6", "n6")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after unlink: got %d, want 401", rec.Code)
	}
	if rec := do(h, "POST", "/auth/apple", body("apple-2", "mum@example.com", "n7", "n7")); rec.Code != http.StatusOK {
		t.Fatalf("email match: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/auth/apple", body("apple-2", "", "n8", "n8")); rec.Code != http.StatusOK {
		t.Fatalf("second sign-in with linked sub: %d %s", rec.Code, rec.Body)
	}

	// A token for some other app is refused.
	bad := appleClaims("apple-3", "mum@example.com", "n9")
	bad["aud"] = "com.someone.else"
	if rec := do(h, "POST", "/auth/apple", jsonBody(t, map[string]any{"identityToken": iss.token(t, bad), "nonce": "n9"})); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong audience: got %d, want 401", rec.Code)
	}
}

func mustMember(t *testing.T, h http.Handler, c *http.Cookie) string {
	t.Helper()
	var m struct{ ID string }
	json.NewDecoder(do(h, "GET", "/me", "", c).Body).Decode(&m)
	return m.ID
}

func TestAuthConfig(t *testing.T) {
	_, v := newAppleIssuer(t)
	rec := do(NewWithStore(t.Context(), family.NewStore(), nil, WithApple(v, "web.service"), WithPasskeys("app.test", "https://app.test")), "GET", "/auth/config", "")
	if !strings.Contains(rec.Body.String(), `"passkeys":true`) || !strings.Contains(rec.Body.String(), `"appleClientId":"web.service"`) {
		t.Fatalf("got %s", rec.Body)
	}
	rec = do(NewWithStore(t.Context(), family.NewStore(), nil), "GET", "/auth/config", "")
	if !strings.Contains(rec.Body.String(), `"passkeys":false`) {
		t.Fatalf("got %s", rec.Body)
	}
	if rec := do(NewWithStore(t.Context(), family.NewStore(), nil), "POST", "/auth/passkey/begin", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("unconfigured passkeys: got %d, want 400", rec.Code)
	}
}
