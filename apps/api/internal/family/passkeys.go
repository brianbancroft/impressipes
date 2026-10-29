package family

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
)

// ErrAppleTaken means the Apple ID already belongs to another member.
var ErrAppleTaken = errors.New("that Apple ID is already linked to another member")

// Passkey is a WebAuthn credential registered by a member.
type Passkey struct {
	ID             []byte
	Name           string
	PublicKey      []byte
	SignCount      uint32
	Transports     []string
	BackupEligible bool
	BackupState    bool
	AAGUID         []byte
	CreatedAt      time.Time
	LastUsedAt     time.Time // zero until first used
}

// AppleID is a member's linked Apple ID.
type AppleID struct {
	Sub, Email string
	LinkedAt   time.Time
}

// ---- in-memory ----

type memPasskey struct {
	Passkey
	member string
}

func (s *Store) SignIn(_ context.Context, memberID string) (*Member, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[memberID]
	if !ok {
		return nil, "", ErrNotFound
	}
	if m.Status == StatusDisabled {
		return nil, "", ErrDisabled
	}
	cp := *m
	return &cp, s.newSession(m), nil
}

func (s *Store) MemberByEmail(_ context.Context, email string) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.byEmail(email); m != nil && email != "" {
		cp := *m
		return &cp, nil
	}
	return nil, ErrNotFound
}

func (s *Store) AddPasskey(_ context.Context, memberID string, p Passkey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.members[memberID]; !ok {
		return ErrNotFound
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = s.now()
	}
	s.passkeys = append(s.passkeys, memPasskey{p, memberID})
	return nil
}

func (s *Store) Passkeys(_ context.Context, memberID string) ([]Passkey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Passkey{}
	for _, p := range s.passkeys {
		if p.member == memberID {
			out = append(out, p.Passkey)
		}
	}
	return out, nil
}

func (s *Store) PasskeyOwner(_ context.Context, id []byte) (*Passkey, *Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.passkeys {
		if bytes.Equal(p.ID, id) {
			m := *s.members[p.member]
			pk := p.Passkey
			return &pk, &m, nil
		}
	}
	return nil, nil, ErrNotFound
}

func (s *Store) TouchPasskey(_ context.Context, id []byte, signCount uint32, backedUp bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.passkeys {
		if bytes.Equal(p.ID, id) {
			s.passkeys[i].SignCount, s.passkeys[i].BackupState, s.passkeys[i].LastUsedAt = signCount, backedUp, s.now()
			return nil
		}
	}
	return ErrNotFound
}

func (s *Store) DeletePasskey(_ context.Context, memberID string, id []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.passkeys {
		if p.member == memberID && bytes.Equal(p.ID, id) {
			s.passkeys = append(s.passkeys[:i], s.passkeys[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (s *Store) AppleLink(_ context.Context, memberID, sub, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.members[memberID]; !ok {
		return ErrNotFound
	}
	if a, ok := s.apple[sub]; ok && a.member != memberID {
		return ErrAppleTaken
	}
	for k, a := range s.apple {
		if a.member == memberID {
			delete(s.apple, k)
		}
	}
	s.apple[sub] = memApple{member: memberID, AppleID: AppleID{Sub: sub, Email: email, LinkedAt: s.now()}}
	return nil
}

func (s *Store) AppleMember(_ context.Context, sub string) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apple[sub]
	if !ok {
		return nil, ErrNotFound
	}
	m := *s.members[a.member]
	return &m, nil
}

func (s *Store) AppleIdentity(_ context.Context, memberID string) (*AppleID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.apple {
		if a.member == memberID {
			id := a.AppleID
			return &id, nil
		}
	}
	return nil, ErrNotFound
}

func (s *Store) AppleUnlink(_ context.Context, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, a := range s.apple {
		if a.member == memberID {
			delete(s.apple, k)
			return nil
		}
	}
	return ErrNotFound
}

type memApple struct {
	AppleID
	member string
}

// ---- Postgres ----

func (p *PG) SignIn(ctx context.Context, memberID string) (*Member, string, error) {
	uid, ok := parseUUID(memberID)
	if !ok {
		return nil, "", ErrNotFound
	}
	u, err := p.q.GetUser(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	} else if err != nil {
		return nil, "", err
	}
	if u.DisabledAt.Valid {
		return nil, "", ErrDisabled
	}
	sess, err := p.session(ctx, p.q, u.ID)
	if err != nil {
		return nil, "", err
	}
	m := memberOf(u)
	return &m, sess, nil
}

func (p *PG) MemberByEmail(ctx context.Context, email string) (*Member, error) {
	u, err := p.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	m := memberOf(u)
	return &m, nil
}

func passkeyOf(r dbgen.Passkey) Passkey {
	pk := Passkey{
		ID: r.CredentialID, Name: r.Name, PublicKey: r.PublicKey, SignCount: uint32(r.SignCount),
		Transports: r.Transports, BackupEligible: r.BackupEligible, BackupState: r.BackupState,
		AAGUID: r.Aaguid, CreatedAt: r.CreatedAt.Time,
	}
	if r.LastUsedAt.Valid {
		pk.LastUsedAt = r.LastUsedAt.Time
	}
	return pk
}

func (p *PG) AddPasskey(ctx context.Context, memberID string, k Passkey) error {
	uid, ok := parseUUID(memberID)
	if !ok {
		return ErrNotFound
	}
	if k.Transports == nil {
		k.Transports = []string{}
	}
	return p.q.AddWebauthnPasskey(ctx, dbgen.AddWebauthnPasskeyParams{
		CredentialID: k.ID, UserID: uid, PublicKey: k.PublicKey, SignCount: int64(k.SignCount),
		Transports: k.Transports, Name: k.Name, BackupEligible: k.BackupEligible,
		BackupState: k.BackupState, Aaguid: k.AAGUID,
	})
}

func (p *PG) Passkeys(ctx context.Context, memberID string) ([]Passkey, error) {
	uid, ok := parseUUID(memberID)
	if !ok {
		return nil, ErrNotFound
	}
	rows, err := p.q.ListWebauthnPasskeys(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]Passkey, 0, len(rows))
	for _, r := range rows {
		out = append(out, passkeyOf(r))
	}
	return out, nil
}

func (p *PG) PasskeyOwner(ctx context.Context, id []byte) (*Passkey, *Member, error) {
	r, err := p.q.GetWebauthnPasskey(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	} else if err != nil {
		return nil, nil, err
	}
	u, err := p.q.GetUser(ctx, r.UserID)
	if err != nil {
		return nil, nil, err
	}
	pk, m := passkeyOf(r), memberOf(u)
	return &pk, &m, nil
}

func (p *PG) TouchPasskey(ctx context.Context, id []byte, signCount uint32, backedUp bool) error {
	return p.q.TouchPasskey(ctx, dbgen.TouchPasskeyParams{CredentialID: id, SignCount: int64(signCount), BackupState: backedUp})
}

func (p *PG) DeletePasskey(ctx context.Context, memberID string, id []byte) error {
	uid, ok := parseUUID(memberID)
	if !ok {
		return ErrNotFound
	}
	n, err := p.q.DeleteWebauthnPasskey(ctx, dbgen.DeleteWebauthnPasskeyParams{CredentialID: id, UserID: uid})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PG) AppleLink(ctx context.Context, memberID, sub, email string) error {
	uid, ok := parseUUID(memberID)
	if !ok {
		return ErrNotFound
	}
	err := p.q.LinkApple(ctx, dbgen.LinkAppleParams{Sub: sub, UserID: uid, Email: email})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return ErrAppleTaken
	}
	return err
}

func (p *PG) AppleMember(ctx context.Context, sub string) (*Member, error) {
	u, err := p.q.GetUserByApple(ctx, sub)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	m := memberOf(u)
	return &m, nil
}

func (p *PG) AppleIdentity(ctx context.Context, memberID string) (*AppleID, error) {
	uid, ok := parseUUID(memberID)
	if !ok {
		return nil, ErrNotFound
	}
	r, err := p.q.GetAppleIdentity(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &AppleID{Sub: r.Sub, Email: r.Email, LinkedAt: r.CreatedAt.Time}, nil
}

func (p *PG) AppleUnlink(ctx context.Context, memberID string) error {
	uid, ok := parseUUID(memberID)
	if !ok {
		return ErrNotFound
	}
	n, err := p.q.UnlinkApple(ctx, uid)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
