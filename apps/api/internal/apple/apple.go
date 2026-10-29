// Package apple verifies Sign in with Apple identity tokens.
package apple

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	Issuer  = "https://appleid.apple.com"
	keysURL = Issuer + "/auth/keys"
)

var ErrInvalidToken = errors.New("invalid Apple identity token")

// Claims is what the app needs from a verified token.
type Claims struct {
	Sub           string
	Email         string
	EmailVerified bool
}

// KeyFunc returns Apple's current signing keys by key id.
type KeyFunc func(ctx context.Context) (map[string]*rsa.PublicKey, error)

// Verifier checks identity tokens issued to any of Audiences: the iOS bundle ID
// for the native flow and the Services ID for the web flow.
type Verifier struct {
	Audiences []string
	Keys      KeyFunc
	Now       func() time.Time
}

// New returns a Verifier that fetches Apple's public keys over HTTPS and caches them.
func New(audiences ...string) *Verifier {
	return &Verifier{Audiences: audiences, Keys: (&keyCache{client: &http.Client{Timeout: 10 * time.Second}}).fetch}
}

// Verify checks the token's signature, issuer, audience, expiry and nonce. rawNonce
// is the value the client generated; the token carries its SHA-256.
func (v *Verifier) Verify(ctx context.Context, token, rawNonce string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || rawNonce == "" {
		return nil, ErrInvalidToken
	}
	var hdr struct{ Alg, Kid string }
	if err := decodePart(parts[0], &hdr); err != nil || hdr.Alg != "RS256" {
		return nil, ErrInvalidToken
	}
	keys, err := v.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("apple keys: %w", err)
	}
	key := keys[hdr.Kid]
	if key == nil {
		return nil, ErrInvalidToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
		return nil, ErrInvalidToken
	}

	var c struct {
		Iss, Sub, Nonce, Email string
		Aud                    audience
		Exp                    float64
		EmailVerified          flexBool `json:"email_verified"`
	}
	if err := decodePart(parts[1], &c); err != nil {
		return nil, ErrInvalidToken
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	if c.Iss != Issuer || c.Sub == "" || !now().Before(time.Unix(int64(c.Exp), 0)) {
		return nil, ErrInvalidToken
	}
	okAud := false
	for _, a := range v.Audiences {
		for _, got := range c.Aud {
			if a != "" && got == a {
				okAud = true
			}
		}
	}
	if !okAud {
		return nil, ErrInvalidToken
	}
	h := sha256.Sum256([]byte(rawNonce))
	if subtle.ConstantTimeCompare([]byte(c.Nonce), []byte(hex.EncodeToString(h[:]))) != 1 {
		return nil, ErrInvalidToken
	}
	return &Claims{Sub: c.Sub, Email: strings.ToLower(c.Email), EmailVerified: bool(c.EmailVerified)}, nil
}

func decodePart(s string, into any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

// audience accepts the JWT "aud" claim as a string or a list.
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		*a = audience{one}
		return nil
	}
	return json.Unmarshal(b, (*[]string)(a))
}

// flexBool accepts true, "true" and "false": Apple sends the string form.
type flexBool bool

func (f *flexBool) UnmarshalJSON(b []byte) error {
	*f = string(b) == "true" || string(b) == `"true"`
	return nil
}

type keyCache struct {
	client  *http.Client
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// fetch returns cached keys for an hour. It does not refetch sooner on a miss for an
// unknown key id, so a flood of bad tokens cannot turn into a flood of requests.
func (c *keyCache) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys != nil && time.Since(c.fetched) < time.Hour {
		return c.keys, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keysURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	var set struct {
		Keys []struct{ Kty, Kid, N, E string }
	}
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("no usable keys")
	}
	c.keys, c.fetched = keys, time.Now()
	return keys, nil
}
