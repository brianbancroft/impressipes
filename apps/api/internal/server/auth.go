package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

type ipKey struct{}

// limiter is a sliding-window counter keyed by an arbitrary string. It lives in
// memory, which is enough for a single-instance deployment.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{hits: map[string][]time.Time{}} }

// allow records a hit for key and reports whether it is within max hits per window.
func (l *limiter) allow(key string, max int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 10_000 {
		for k, v := range l.hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > time.Hour {
				delete(l.hits, k)
			}
		}
	}
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= max {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}

// clientIP is the address the request came from. Behind the Coolify proxy the
// last X-Forwarded-For entry is the one the proxy itself appended.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *server) ip(ctx context.Context) string {
	ip, _ := ctx.Value(ipKey{}).(string)
	return ip
}

const authWindow = 15 * time.Minute

func sessionCookieHeader(sess string) string {
	return (&http.Cookie{
		Name: sessionCookie, Value: sess, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 365,
	}).String()
}

// sendChallenge emails a link and code. It runs in the background so the
// response time does not reveal whether the address belongs to a member.
func (s *server) sendChallenge(c family.Challenge) {
	link := s.baseURL + "/auth/verify?t=" + c.Token
	var subject, intro string
	if c.Purpose == family.PurposeVerify {
		subject = "Confirm your email for Impressipes"
		intro = "Confirm this email address for your Impressipes account."
	} else {
		subject = "Your Impressipes sign-in link"
		intro = "Sign in to Impressipes."
	}
	body := "Hi " + c.Name + ",\r\n\r\n" + intro + "\r\n\r\n" +
		"Open this link:\r\n" + link + "\r\n\r\n" +
		"Or enter this code in the app: " + c.Code + "\r\n\r\n" +
		"Both work once and expire in 15 minutes. If you didn't ask for this, you can ignore this email.\r\n"
	if s.mailer == nil {
		log.Printf("mail not configured; %s for %s: %s (code %s)", c.Purpose, c.Email, link, c.Code)
		return
	}
	go func() {
		if err := s.mailer.Send(c.Email, subject, body); err != nil {
			log.Printf("%s email to %s failed: %v", c.Purpose, c.Email, err)
		}
	}()
}

func (s *server) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	if req.Body == nil {
		return api.Login401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("invalid body"))}, nil
	}
	email := strings.ToLower(strings.TrimSpace(req.Body.Email))
	if !s.limits.allow("login-ip:"+s.ip(ctx), 30, authWindow) || !s.limits.allow("login:"+email, 10, authWindow) {
		return api.Login429JSONResponse(errBody("too many attempts; try again later")), nil
	}
	m, sess, err := s.store.PasswordLogin(ctx, email, req.Body.Password)
	switch {
	case errors.Is(err, family.ErrBadCredentials):
		return api.Login401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(err.Error()))}, nil
	case errors.Is(err, family.ErrDisabled):
		return api.Login403JSONResponse(errBody("disabled")), nil
	case err != nil:
		return nil, err
	}
	return api.Login200JSONResponse{Body: toMember(*m), Headers: api.Login200ResponseHeaders{SetCookie: sessionCookieHeader(sess)}}, nil
}

func (s *server) RequestMagicLink(ctx context.Context, req api.RequestMagicLinkRequestObject) (api.RequestMagicLinkResponseObject, error) {
	if req.Body == nil || !s.limits.allow("magic-ip:"+s.ip(ctx), 10, time.Hour) {
		return api.RequestMagicLink429JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("too many requests; try again later"))}, nil
	}
	c, err := s.store.StartLogin(ctx, req.Body.Email)
	if err != nil {
		return nil, err
	}
	if c != nil {
		s.sendChallenge(*c)
	} else {
		log.Print("sign-in link requested for an address with no confirmed, active member (or rate limited); nothing sent")
	}
	return api.RequestMagicLink202Response{}, nil
}

func (s *server) VerifyMagicLink(ctx context.Context, req api.VerifyMagicLinkRequestObject) (api.VerifyMagicLinkResponseObject, error) {
	b := req.Body
	if b == nil || (b.Token == nil && (b.Email == nil || b.Code == nil)) {
		return api.VerifyMagicLink400JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("send a token, or an email and code"))}, nil
	}
	if !s.limits.allow("verify-ip:"+s.ip(ctx), 30, authWindow) {
		return api.VerifyMagicLink429JSONResponse(errBody("too many attempts; try again later")), nil
	}
	var (
		m    *family.Member
		sess string
		err  error
	)
	if b.Token != nil {
		m, sess, err = s.store.CompleteLink(ctx, *b.Token)
	} else {
		email := strings.ToLower(strings.TrimSpace(*b.Email))
		if !s.limits.allow("verify:"+email, 10, authWindow) {
			return api.VerifyMagicLink429JSONResponse(errBody("too many attempts; try again later")), nil
		}
		m, sess, err = s.store.CompleteCode(ctx, email, strings.TrimSpace(*b.Code))
	}
	switch {
	case errors.Is(err, family.ErrInvalidLogin):
		return api.VerifyMagicLink401JSONResponse(errBody(err.Error())), nil
	case errors.Is(err, family.ErrDisabled):
		return api.VerifyMagicLink403JSONResponse(errBody("disabled")), nil
	case errors.Is(err, family.ErrEmailTaken):
		return api.VerifyMagicLink409JSONResponse(errBody(err.Error())), nil
	case err != nil:
		return nil, err
	}
	return api.VerifyMagicLink200JSONResponse{Body: toMember(*m), Headers: api.VerifyMagicLink200ResponseHeaders{SetCookie: sessionCookieHeader(sess)}}, nil
}

func (s *server) SetEmail(ctx context.Context, req api.SetEmailRequestObject) (api.SetEmailResponseObject, error) {
	m, fail := s.current(ctx)
	if fail != nil {
		if fail.code == http.StatusForbidden {
			return api.SetEmail403JSONResponse(errBody(fail.msg)), nil
		}
		return api.SetEmail401JSONResponse(errBody(fail.msg)), nil
	}
	if req.Body == nil || !s.limits.allow("email:"+m.ID, 5, time.Hour) {
		return api.SetEmail429JSONResponse(errBody("too many requests; try again later")), nil
	}
	c, err := s.store.StartVerify(ctx, m.ID, req.Body.Email)
	switch {
	case errors.Is(err, family.ErrInvalidEmail):
		return api.SetEmail400JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("that doesn't look like an email address"))}, nil
	case errors.Is(err, family.ErrEmailTaken):
		return api.SetEmail409JSONResponse(errBody(err.Error())), nil
	case errors.Is(err, family.ErrTooMany):
		return api.SetEmail429JSONResponse(errBody("too many requests; try again later")), nil
	case err != nil:
		return nil, err
	}
	s.sendChallenge(*c)
	return api.SetEmail202Response{}, nil
}

func (s *server) SetPassword(ctx context.Context, req api.SetPasswordRequestObject) (api.SetPasswordResponseObject, error) {
	m, fail := s.current(ctx)
	if fail != nil {
		if fail.code == http.StatusForbidden {
			return api.SetPassword403JSONResponse(errBody(fail.msg)), nil
		}
		return api.SetPassword401JSONResponse(errBody(fail.msg)), nil
	}
	if req.Body == nil {
		return api.SetPassword400JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("invalid body"))}, nil
	}
	switch err := s.store.SetPassword(ctx, m.ID, req.Body.Password); {
	case errors.Is(err, family.ErrWeakPassword):
		return api.SetPassword400JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(err.Error()))}, nil
	case err != nil:
		return nil, err
	}
	return api.SetPassword204Response{}, nil
}

// IOSBundleID is the iOS app's bundle identifier, which is also the audience of its Apple identity tokens.
const IOSBundleID = "ca.brianbancroft.impressipies"

// appSiteAssociation tells iOS that the app may open the emailed sign-in link and
// use this site's passkeys.
func appSiteAssociation(team string) http.HandlerFunc {
	id := team + "." + IOSBundleID
	body := `{"applinks":{"details":[{"appIDs":["` + id + `"],"components":[{"/":"/auth/verify"}]}]},` +
		`"webcredentials":{"apps":["` + id + `"]}}`
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}
}
