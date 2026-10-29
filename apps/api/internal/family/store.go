// Package family holds the in-memory household model: members, invites and sessions.
package family

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	RoleAdmin  = "admin"
	RoleMember = "member"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

const InviteTTL = 7 * 24 * time.Hour

var (
	ErrNotFound   = errors.New("not found")
	ErrInviteUsed = errors.New("invite already used")
	ErrExpired    = errors.New("invite expired")
	ErrDisabled   = errors.New("member disabled")
)

type Member struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	CredentialID string `json:"-"`
	Email        string `json:"-"`
	HasPassword  bool   `json:"-"`
}

type Invite struct {
	Token     string
	MemberID  string // set for a re-invite of an existing member
	ExpiresAt time.Time
	Used      bool
}

type Store struct {
	mu       sync.Mutex
	now      func() time.Time
	members  map[string]*Member
	invites  map[string]*Invite
	sessions map[string]string

	passwords  map[string]string     // member id → password hash
	challenges map[string]*challenge // link hash → emailed link + code
}

type challenge struct {
	userID, purpose, email, codeHash string
	attempts                         int
	expires, created                 time.Time
	used                             bool
}

func NewStore() *Store {
	return &Store{
		now:      time.Now,
		members:  map[string]*Member{},
		invites:  map[string]*Invite{},
		sessions: map[string]string{},

		passwords:  map[string]string{},
		challenges: map[string]*challenge{},
	}
}

func token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// CreateInvite issues a single-use link. A non-empty memberID re-invites that
// member (lost device), who keeps their identity and replaces their passkey.
func (s *Store) CreateInvite(_ context.Context, memberID string) (*Invite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if memberID != "" {
		m, ok := s.members[memberID]
		if !ok {
			return nil, ErrNotFound
		}
		if m.Status == StatusDisabled {
			return nil, ErrDisabled
		}
	}
	inv := &Invite{Token: token(), MemberID: memberID, ExpiresAt: s.now().Add(InviteTTL)}
	s.invites[inv.Token] = inv
	return inv, nil
}

// Invite looks up an invite and the member it re-invites, if any.
func (s *Store) Invite(_ context.Context, tok string) (*Invite, *Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[tok]
	if !ok {
		return nil, nil, ErrNotFound
	}
	var m *Member
	if inv.MemberID != "" {
		m = s.members[inv.MemberID]
	}
	cp := *inv
	if m != nil {
		mc := *m
		m = &mc
	}
	return &cp, m, nil
}

// Accept consumes an invite, creating (or updating) the member and a session.
// The first member ever created becomes the admin.
func (s *Store) Accept(_ context.Context, tok string, in AcceptInput) (*Member, string, *Challenge, error) {
	email, pwHash, err := prepareCredentials(in)
	if err != nil {
		return nil, "", nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[tok]
	if !ok {
		return nil, "", nil, ErrNotFound
	}
	if inv.Used {
		return nil, "", nil, ErrInviteUsed
	}
	if s.now().After(inv.ExpiresAt) {
		return nil, "", nil, ErrExpired
	}

	var m *Member
	if inv.MemberID != "" {
		m = s.members[inv.MemberID]
		if m.Status == StatusDisabled {
			return nil, "", nil, ErrDisabled
		}
	} else {
		if email == "" || pwHash == "" {
			return nil, "", nil, ErrCredentialsRequired
		}
		role := RoleMember
		if len(s.members) == 0 {
			role = RoleAdmin
		}
		m = &Member{ID: token()[:12], Role: role, Status: StatusActive}
		s.members[m.ID] = m
	}
	if email != "" && email != m.Email {
		if other := s.byEmail(email); other != nil && other.ID != m.ID {
			return nil, "", nil, ErrEmailTaken
		}
	}
	m.Name, m.Color, m.CredentialID = in.Name, in.Color, in.CredentialID
	if pwHash != "" {
		s.passwords[m.ID], m.HasPassword = pwHash, true
	}
	var ch *Challenge
	if email != "" && email != m.Email {
		c := s.addChallenge(m, PurposeVerify, email)
		ch = &c
	}
	inv.Used = true

	sess := s.newSession(m)
	cp := *m
	return &cp, sess, ch, nil
}

// Session resolves a session token to its member.
func (s *Store) Session(_ context.Context, sess string) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.sessions[sess]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s.members[id]
	return &cp, nil
}

func (s *Store) Members(_ context.Context) ([]Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Member, 0, len(s.members))
	for _, m := range s.members {
		out = append(out, *m)
	}
	return out, nil
}

// SetStatus disables or re-enables a member; disabling also drops their sessions.
func (s *Store) SetStatus(_ context.Context, id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return ErrNotFound
	}
	m.Status = status
	if status == StatusDisabled {
		for sess, mid := range s.sessions {
			if mid == id {
				delete(s.sessions, sess)
			}
		}
	}
	return nil
}
