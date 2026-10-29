package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

const sessionCookie = "impressipies_session"

type ctxKey struct{}

type server struct {
	store family.Directory
	pool  *pgxpool.Pool // nil in memory mode, where only the household endpoints work
	q     *dbgen.Queries
	hub   *hub
}

var _ api.StrictServerInterface = (*server)(nil)

// New returns the API's root handler. A nil pool runs on the in-memory directory
// (local development and tests); otherwise everything is persisted to Postgres.
// Until the first member exists it logs a bootstrap admin invite link.
func New(pool *pgxpool.Pool) http.Handler {
	var dir family.Directory = family.NewStore()
	first := true
	if pool != nil {
		pg := family.NewPG(pool)
		dir = pg
		var err error
		if first, err = pg.ReadyForBootstrap(context.Background()); err != nil {
			log.Fatalf("database: %v", err)
		}
	}
	if first {
		if inv, err := dir.CreateInvite(context.Background(), ""); err == nil {
			log.Printf("first run: open /invite/%s to create the admin account", inv.Token)
		}
	}
	return NewWithStore(context.Background(), dir, pool)
}

// NewWithStore builds the handler on a directory and, optionally, a database pool.
// ctx ends the background listener that feeds the event stream.
func NewWithStore(ctx context.Context, store family.Directory, pool *pgxpool.Pool) http.Handler {
	s := &server{store: store, pool: pool}
	if pool != nil {
		s.q = dbgen.New(pool)
		s.hub = newHub()
		go s.hub.listen(ctx, pool)
	}
	strict := api.NewStrictHandlerWithOptions(s, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestError,
		ResponseErrorHandlerFunc: responseError,
	})
	mux := http.NewServeMux()
	// The event stream is not part of the OpenAPI document: a long-lived response
	// does not fit the generated request/response handlers.
	mux.HandleFunc("GET /events", s.events)
	rest := api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		ErrorHandlerFunc: requestError,
	})
	mux.Handle("/", rest)
	return sessionMiddleware(mux)
}

// sessionMiddleware hands the request's session cookie to the strict handlers via the context.
func sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), viaKey{}, clientVia(r)))
		if c, err := r.Cookie(sessionCookie); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, c.Value))
		}
		next.ServeHTTP(w, r)
	})
}

func errBody(msg string) api.Error { return api.Error{Error: msg} }

// authFail is the status and message for a request that cannot proceed as a member.
type authFail struct {
	code int
	msg  string
}

// current resolves the request's session. A disabled member gets 403 so the
// web app can show the disabled state rather than a sign-in prompt.
func (s *server) current(ctx context.Context) (*family.Member, *authFail) {
	sess, _ := ctx.Value(ctxKey{}).(string)
	if sess == "" {
		return nil, &authFail{http.StatusUnauthorized, "unauthenticated"}
	}
	m, err := s.store.Session(ctx, sess)
	if err != nil {
		return nil, &authFail{http.StatusUnauthorized, "unauthenticated"}
	}
	if m.Status == family.StatusDisabled {
		return nil, &authFail{http.StatusForbidden, "disabled"}
	}
	return m, nil
}

func (s *server) admin(ctx context.Context) *authFail {
	m, fail := s.current(ctx)
	if fail != nil {
		return fail
	}
	if m.Role != family.RoleAdmin {
		return &authFail{http.StatusForbidden, "admin only"}
	}
	return nil
}

func toMember(m family.Member) api.Member {
	return api.Member{
		Id: m.ID, Name: m.Name, Color: m.Color,
		Role: api.MemberRole(m.Role), Status: api.MemberStatus(m.Status),
	}
}

func (s *server) GetHealth(context.Context, api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok}, nil
}

func (s *server) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	m, fail := s.current(ctx)
	switch {
	case fail == nil:
		return api.GetMe200JSONResponse(toMember(*m)), nil
	case fail.code == http.StatusForbidden:
		return api.GetMe403JSONResponse(errBody(fail.msg)), nil
	default:
		return api.GetMe401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(fail.msg))}, nil
	}
}

func (s *server) ListMembers(ctx context.Context, _ api.ListMembersRequestObject) (api.ListMembersResponseObject, error) {
	if fail := s.admin(ctx); fail != nil {
		if fail.code == http.StatusForbidden {
			return api.ListMembers403JSONResponse(errBody(fail.msg)), nil
		}
		return api.ListMembers401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(fail.msg))}, nil
	}
	members, err := s.store.Members(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListMembers200JSONResponse, 0, len(members))
	for _, m := range members {
		out = append(out, toMember(m))
	}
	return out, nil
}

func (s *server) DisableMember(ctx context.Context, req api.DisableMemberRequestObject) (api.DisableMemberResponseObject, error) {
	if fail := s.admin(ctx); fail != nil {
		if fail.code == http.StatusForbidden {
			return api.DisableMember403JSONResponse(errBody(fail.msg)), nil
		}
		return api.DisableMember401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(fail.msg))}, nil
	}
	if err := s.store.SetStatus(ctx, req.Id, family.StatusDisabled); err != nil {
		return api.DisableMember404JSONResponse(errBody("member not found")), nil
	}
	return api.DisableMember204Response{}, nil
}

func (s *server) EnableMember(ctx context.Context, req api.EnableMemberRequestObject) (api.EnableMemberResponseObject, error) {
	if fail := s.admin(ctx); fail != nil {
		if fail.code == http.StatusForbidden {
			return api.EnableMember403JSONResponse(errBody(fail.msg)), nil
		}
		return api.EnableMember401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(fail.msg))}, nil
	}
	if err := s.store.SetStatus(ctx, req.Id, family.StatusActive); err != nil {
		return api.EnableMember404JSONResponse(errBody("member not found")), nil
	}
	return api.EnableMember204Response{}, nil
}

func (s *server) CreateInvite(ctx context.Context, req api.CreateInviteRequestObject) (api.CreateInviteResponseObject, error) {
	if fail := s.admin(ctx); fail != nil {
		if fail.code == http.StatusForbidden {
			return api.CreateInvite403JSONResponse(errBody(fail.msg)), nil
		}
		return api.CreateInvite401JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody(fail.msg))}, nil
	}
	memberID := ""
	if req.Body != nil && req.Body.MemberId != nil {
		memberID = *req.Body.MemberId
	}
	inv, err := s.store.CreateInvite(ctx, memberID)
	switch {
	case errors.Is(err, family.ErrNotFound):
		return api.CreateInvite404JSONResponse(errBody("member not found")), nil
	case errors.Is(err, family.ErrDisabled):
		return api.CreateInvite409JSONResponse(errBody("member is disabled; enable them before re-inviting")), nil
	case err != nil:
		return nil, err
	}
	return api.CreateInvite201JSONResponse{Token: inv.Token, Path: "/invite/" + inv.Token}, nil
}

func (s *server) GetInvite(ctx context.Context, req api.GetInviteRequestObject) (api.GetInviteResponseObject, error) {
	inv, m, err := s.store.Invite(ctx, req.Token)
	if err != nil {
		return api.GetInvite404JSONResponse{State: api.InviteInfoStateInvalid}, nil
	}
	state := api.InviteInfoStateValid
	switch {
	case inv.Used:
		state = api.InviteInfoStateUsed
	case !inv.ExpiresAt.After(timeNow()):
		state = api.InviteInfoStateExpired
	case m != nil && m.Status == family.StatusDisabled:
		state = api.InviteInfoStateDisabled
	}
	reinvite := m != nil
	resp := api.GetInvite200JSONResponse{State: state, Reinvite: &reinvite}
	if m != nil {
		resp.Name, resp.Color = &m.Name, &m.Color
	}
	return resp, nil
}

func (s *server) AcceptInvite(ctx context.Context, req api.AcceptInviteRequestObject) (api.AcceptInviteResponseObject, error) {
	if req.Body == nil {
		return api.AcceptInvite400JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("invalid body"))}, nil
	}
	name := strings.TrimSpace(req.Body.Name)
	if name == "" || req.Body.Color == "" || req.Body.CredentialId == "" {
		return api.AcceptInvite400JSONResponse{ErrorJSONResponse: api.ErrorJSONResponse(errBody("name, color and credentialId are required"))}, nil
	}
	m, sess, err := s.store.Accept(ctx, req.Token, name, req.Body.Color, req.Body.CredentialId)
	switch {
	case errors.Is(err, family.ErrNotFound):
		return api.AcceptInvite404JSONResponse(errBody("invite not found")), nil
	case errors.Is(err, family.ErrInviteUsed), errors.Is(err, family.ErrExpired):
		return api.AcceptInvite410JSONResponse(errBody(err.Error())), nil
	case errors.Is(err, family.ErrDisabled):
		return api.AcceptInvite403JSONResponse(errBody("disabled")), nil
	case err != nil:
		return nil, err
	}
	cookie := (&http.Cookie{
		Name: sessionCookie, Value: sess, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 365,
	}).String()
	return api.AcceptInvite201JSONResponse{
		Body:    toMember(*m),
		Headers: api.AcceptInvite201ResponseHeaders{SetCookie: cookie},
	}, nil
}

var timeNow = func() time.Time { return time.Now() }
