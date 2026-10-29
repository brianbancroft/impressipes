package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/apple"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

const (
	ceremonyTTL     = 5 * time.Minute
	kindPasskeyAuth = "login"
	kindPasskeyAdd  = "register"
)

// ceremonies holds the challenge of each passkey ceremony in flight. It lives in
// memory like the rate limiter: a restart only costs someone a retry.
type ceremonies struct {
	mu sync.Mutex
	m  map[string]ceremony
}

type ceremony struct {
	kind, member string
	data         webauthn.SessionData
	expires      time.Time
}

func newCeremonies() *ceremonies { return &ceremonies{m: map[string]ceremony{}} }

func (c *ceremonies) put(kind, member string, data webauthn.SessionData) string {
	id := family.NewToken()
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.m {
		if now.After(v.expires) {
			delete(c.m, k)
		}
	}
	c.m[id] = ceremony{kind, member, data, now.Add(ceremonyTTL)}
	return id
}

// take returns a ceremony once; a second call, or the wrong kind, finds nothing.
func (c *ceremonies) take(id, kind string) (ceremony, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[id]
	delete(c.m, id)
	return v, ok && v.kind == kind && time.Now().Before(v.expires)
}

// waUser adapts a member to the webauthn library.
type waUser struct {
	m     family.Member
	creds []webauthn.Credential
}

func (u waUser) WebAuthnID() []byte { return []byte(u.m.ID) }
func (u waUser) WebAuthnName() string {
	if u.m.Email != "" {
		return u.m.Email
	}
	return u.m.Name
}
func (u waUser) WebAuthnDisplayName() string                { return u.m.Name }
func (u waUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func toCredential(p family.Passkey) webauthn.Credential {
	tr := make([]protocol.AuthenticatorTransport, len(p.Transports))
	for i, t := range p.Transports {
		tr[i] = protocol.AuthenticatorTransport(t)
	}
	return webauthn.Credential{
		ID: p.ID, PublicKey: p.PublicKey, Transport: tr,
		Flags: webauthn.CredentialFlags{
			UserPresent: true, UserVerified: true, BackupEligible: p.BackupEligible, BackupState: p.BackupState,
		},
		Authenticator: webauthn.Authenticator{AAGUID: p.AAGUID, SignCount: p.SignCount},
	}
}

func (s *server) userWithPasskeys(ctx context.Context, m family.Member) (waUser, error) {
	keys, err := s.store.Passkeys(ctx, m.ID)
	if err != nil {
		return waUser{}, err
	}
	u := waUser{m: m}
	for _, k := range keys {
		u.creds = append(u.creds, toCredential(k))
	}
	return u, nil
}

var b64 = base64.RawURLEncoding

// asObject turns a library value into the generic JSON object the API carries.
func asObject(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(raw, &out)
}

// signedIn is the signed-in member, or an error that answers 401 or 403.
func (s *server) signedIn(ctx context.Context) (*family.Member, error) {
	m, f := s.current(ctx)
	if f != nil {
		return nil, fail(f.code, f.msg)
	}
	return m, nil
}

func (s *server) needPasskeys() error {
	if s.wa == nil {
		return fail(http.StatusBadRequest, "passkeys are not set up on this server")
	}
	return nil
}

func (s *server) GetAuthConfig(context.Context, api.GetAuthConfigRequestObject) (api.GetAuthConfigResponseObject, error) {
	res := api.GetAuthConfig200JSONResponse{Passkeys: s.wa != nil}
	if s.apple != nil && s.appleWeb != "" {
		res.AppleClientId = &s.appleWeb
	}
	return res, nil
}

func (s *server) BeginPasskeyLogin(ctx context.Context, _ api.BeginPasskeyLoginRequestObject) (api.BeginPasskeyLoginResponseObject, error) {
	if err := s.needPasskeys(); err != nil {
		return nil, err
	}
	if !s.limits.allow("passkey-ip:"+s.ip(ctx), 60, authWindow) {
		return nil, fail(http.StatusTooManyRequests, "too many attempts; try again later")
	}
	opts, data, err := s.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, err
	}
	obj, err := asObject(opts)
	if err != nil {
		return nil, err
	}
	return api.BeginPasskeyLogin200JSONResponse{CeremonyId: s.ceremonies.put(kindPasskeyAuth, "", *data), Options: obj}, nil
}

func (s *server) FinishPasskeyLogin(ctx context.Context, req api.FinishPasskeyLoginRequestObject) (api.FinishPasskeyLoginResponseObject, error) {
	if err := s.needPasskeys(); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	if !s.limits.allow("passkey-ip:"+s.ip(ctx), 60, authWindow) {
		return nil, fail(http.StatusTooManyRequests, "too many attempts; try again later")
	}
	c, ok := s.ceremonies.take(req.Body.CeremonyId, kindPasskeyAuth)
	if !ok {
		return nil, fail(http.StatusUnauthorized, "that sign-in expired; try again")
	}
	raw, _ := json.Marshal(req.Body.Credential)
	parsed, err := protocol.ParseCredentialRequestResponseBytes(raw)
	if err != nil {
		return nil, fail(http.StatusBadRequest, "that isn't a valid passkey response")
	}
	var owner *family.Member
	user, cred, err := s.wa.ValidatePasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		_, m, err := s.store.PasskeyOwner(ctx, rawID)
		if err != nil {
			return nil, err
		}
		owner = m
		return s.userWithPasskeys(ctx, *m)
	}, c.data, parsed)
	if err != nil || owner == nil || string(user.WebAuthnID()) != owner.ID {
		return nil, fail(http.StatusUnauthorized, "that passkey isn't registered here")
	}
	if err := s.store.TouchPasskey(ctx, cred.ID, cred.Authenticator.SignCount, cred.Flags.BackupState); err != nil {
		log.Printf("passkey touch: %v", err)
	}
	m, sess, err := s.store.SignIn(ctx, owner.ID)
	if errors.Is(err, family.ErrDisabled) {
		return nil, fail(http.StatusForbidden, "disabled")
	} else if err != nil {
		return nil, err
	}
	return api.FinishPasskeyLogin200JSONResponse{Body: toMember(*m), Headers: api.FinishPasskeyLogin200ResponseHeaders{SetCookie: sessionCookieHeader(sess)}}, nil
}

func (s *server) BeginPasskeyRegistration(ctx context.Context, _ api.BeginPasskeyRegistrationRequestObject) (api.BeginPasskeyRegistrationResponseObject, error) {
	if err := s.needPasskeys(); err != nil {
		return nil, err
	}
	m, err := s.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if !s.limits.allow("passkey-add:"+m.ID, 20, time.Hour) {
		return nil, fail(http.StatusTooManyRequests, "too many attempts; try again later")
	}
	u, err := s.userWithPasskeys(ctx, *m)
	if err != nil {
		return nil, err
	}
	exclude := make([]protocol.CredentialDescriptor, len(u.creds))
	for i, c := range u.creds {
		exclude[i] = c.Descriptor()
	}
	opts, data, err := s.wa.BeginRegistration(u,
		webauthn.WithExclusions(exclude),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return nil, err
	}
	obj, err := asObject(opts)
	if err != nil {
		return nil, err
	}
	return api.BeginPasskeyRegistration200JSONResponse{CeremonyId: s.ceremonies.put(kindPasskeyAdd, m.ID, *data), Options: obj}, nil
}

func (s *server) FinishPasskeyRegistration(ctx context.Context, req api.FinishPasskeyRegistrationRequestObject) (api.FinishPasskeyRegistrationResponseObject, error) {
	if err := s.needPasskeys(); err != nil {
		return nil, err
	}
	m, err := s.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	c, ok := s.ceremonies.take(req.Body.CeremonyId, kindPasskeyAdd)
	if !ok || c.member != m.ID {
		return nil, fail(http.StatusBadRequest, "that request expired; try again")
	}
	raw, _ := json.Marshal(req.Body.Credential)
	parsed, err := protocol.ParseCredentialCreationResponseBytes(raw)
	if err != nil {
		return nil, fail(http.StatusBadRequest, "that isn't a valid passkey")
	}
	u, err := s.userWithPasskeys(ctx, *m)
	if err != nil {
		return nil, err
	}
	cred, err := s.wa.CreateCredential(u, c.data, parsed)
	if err != nil {
		return nil, fail(http.StatusBadRequest, "couldn't verify that passkey")
	}
	name := "Passkey"
	if req.Body.Name != nil && *req.Body.Name != "" {
		name = truncate(*req.Body.Name, 60)
	}
	tr := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		tr[i] = string(t)
	}
	pk := family.Passkey{
		ID: cred.ID, Name: name, PublicKey: cred.PublicKey, SignCount: cred.Authenticator.SignCount, Transports: tr,
		BackupEligible: cred.Flags.BackupEligible, BackupState: cred.Flags.BackupState, AAGUID: cred.Authenticator.AAGUID,
		CreatedAt: time.Now(),
	}
	if err := s.store.AddPasskey(ctx, m.ID, pk); err != nil {
		return nil, err
	}
	return api.FinishPasskeyRegistration201JSONResponse(toPasskey(pk)), nil
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func toPasskey(p family.Passkey) api.Passkey {
	out := api.Passkey{Id: b64.EncodeToString(p.ID), Name: p.Name, CreatedAt: p.CreatedAt, Synced: p.BackupState}
	if !p.LastUsedAt.IsZero() {
		out.LastUsedAt = &p.LastUsedAt
	}
	return out
}

func (s *server) GetSignInMethods(ctx context.Context, _ api.GetSignInMethodsRequestObject) (api.GetSignInMethodsResponseObject, error) {
	m, err := s.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := s.store.Passkeys(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	res := api.GetSignInMethods200JSONResponse{Passkeys: make([]api.Passkey, len(keys))}
	for i, k := range keys {
		res.Passkeys[i] = toPasskey(k)
	}
	switch a, err := s.store.AppleIdentity(ctx, m.ID); {
	case err == nil:
		res.Apple = &api.AppleLink{Email: strPtr(a.Email)}
	case !errors.Is(err, family.ErrNotFound):
		return nil, err
	}
	return res, nil
}

// otherMethods counts the ways m could still sign in after dropping one passkey or
// the Apple ID, so nobody removes their last way in.
func (s *server) otherMethods(ctx context.Context, m *family.Member, dropPasskey []byte, dropApple bool) (int, error) {
	n := 0
	if m.Email != "" {
		n++
	}
	if m.HasPassword {
		n++
	}
	keys, err := s.store.Passkeys(ctx, m.ID)
	if err != nil {
		return 0, err
	}
	for _, k := range keys {
		if string(k.ID) != string(dropPasskey) {
			n++
		}
	}
	if _, err := s.store.AppleIdentity(ctx, m.ID); err == nil && !dropApple {
		n++
	} else if err != nil && !errors.Is(err, family.ErrNotFound) {
		return 0, err
	}
	return n, nil
}

func (s *server) DeletePasskey(ctx context.Context, req api.DeletePasskeyRequestObject) (api.DeletePasskeyResponseObject, error) {
	m, err := s.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	id, err := b64.DecodeString(req.PasskeyId)
	if err != nil {
		return nil, fail(http.StatusNotFound, "passkey not found")
	}
	n, err := s.otherMethods(ctx, m, id, false)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fail(http.StatusConflict, "that's your only way to sign in; add an email, password or another passkey first")
	}
	switch err := s.store.DeletePasskey(ctx, m.ID, id); {
	case errors.Is(err, family.ErrNotFound):
		return nil, fail(http.StatusNotFound, "passkey not found")
	case err != nil:
		return nil, err
	}
	return api.DeletePasskey204Response{}, nil
}

// ---- Sign in with Apple ----

// appleClaims verifies a token from the client. A token Apple's keys don't vouch for
// answers with the status rejected: 401 where signing in, but 400 for a signed-in member
// linking an account, since the iOS app treats any 401 as a lapsed session.
func (s *server) appleClaims(ctx context.Context, t *api.AppleToken, rejected int) (*apple.Claims, error) {
	if s.apple == nil {
		return nil, fail(http.StatusBadRequest, "Sign in with Apple is not set up on this server")
	}
	if t == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	c, err := s.apple.Verify(ctx, t.IdentityToken, t.Nonce)
	switch {
	case errors.Is(err, apple.ErrInvalidToken):
		return nil, fail(rejected, "Apple didn't accept that sign-in; try again")
	case err != nil:
		log.Printf("apple verify: %v", err)
		return nil, fail(http.StatusBadGateway, "couldn't reach Apple; try again")
	}
	return c, nil
}

func (s *server) SignInWithApple(ctx context.Context, req api.SignInWithAppleRequestObject) (api.SignInWithAppleResponseObject, error) {
	if !s.limits.allow("apple-ip:"+s.ip(ctx), 30, authWindow) {
		return nil, fail(http.StatusTooManyRequests, "too many attempts; try again later")
	}
	c, err := s.appleClaims(ctx, req.Body, http.StatusUnauthorized)
	if err != nil {
		return nil, err
	}
	m, err := s.store.AppleMember(ctx, c.Sub)
	if errors.Is(err, family.ErrNotFound) {
		m, err = s.matchByVerifiedEmail(ctx, c)
	}
	if err != nil {
		return nil, err
	}
	m, sess, err := s.store.SignIn(ctx, m.ID)
	if errors.Is(err, family.ErrDisabled) {
		return nil, fail(http.StatusForbidden, "disabled")
	} else if err != nil {
		return nil, err
	}
	return api.SignInWithApple200JSONResponse{Body: toMember(*m), Headers: api.SignInWithApple200ResponseHeaders{SetCookie: sessionCookieHeader(sess)}}, nil
}

// matchByVerifiedEmail links an Apple ID the first time it signs in, when Apple vouches
// for an address that is also a member's confirmed email and that member has no Apple ID yet.
func (s *server) matchByVerifiedEmail(ctx context.Context, c *apple.Claims) (*family.Member, error) {
	unknown := fail(http.StatusUnauthorized, "This Apple ID isn't linked to a member yet. Sign in another way, then connect Apple under Account.")
	if !c.EmailVerified || c.Email == "" {
		return nil, unknown
	}
	m, err := s.store.MemberByEmail(ctx, c.Email)
	if errors.Is(err, family.ErrNotFound) {
		return nil, unknown
	} else if err != nil {
		return nil, err
	}
	if _, err := s.store.AppleIdentity(ctx, m.ID); err == nil {
		return nil, unknown
	} else if !errors.Is(err, family.ErrNotFound) {
		return nil, err
	}
	if err := s.store.AppleLink(ctx, m.ID, c.Sub, c.Email); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *server) LinkApple(ctx context.Context, req api.LinkAppleRequestObject) (api.LinkAppleResponseObject, error) {
	m, err := s.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	c, err := s.appleClaims(ctx, req.Body, http.StatusBadRequest)
	if err != nil {
		return nil, err
	}
	switch err := s.store.AppleLink(ctx, m.ID, c.Sub, c.Email); {
	case errors.Is(err, family.ErrAppleTaken):
		return nil, fail(http.StatusConflict, err.Error())
	case err != nil:
		return nil, err
	}
	return api.LinkApple200JSONResponse{Email: strPtr(c.Email)}, nil
}

func (s *server) UnlinkApple(ctx context.Context, _ api.UnlinkAppleRequestObject) (api.UnlinkAppleResponseObject, error) {
	m, err := s.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	n, err := s.otherMethods(ctx, m, nil, true)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fail(http.StatusConflict, "that's your only way to sign in; add an email, password or a passkey first")
	}
	switch err := s.store.AppleUnlink(ctx, m.ID); {
	case errors.Is(err, family.ErrNotFound):
		return nil, fail(http.StatusNotFound, "no Apple ID is linked")
	case err != nil:
		return nil, err
	}
	return api.UnlinkApple204Response{}, nil
}
