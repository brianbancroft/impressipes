package family

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

// SessionTTL is how long a signed-in device stays signed in.
const SessionTTL = 365 * 24 * time.Hour

// PG is the Postgres-backed Directory.
type PG struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

func NewPG(pool *pgxpool.Pool) *PG { return &PG{pool: pool, q: dbgen.New(pool)} }

func uuidString(u pgtype.UUID) string { return uuidx.String(u) }

func parseUUID(s string) (pgtype.UUID, bool) { return uuidx.Parse(s) }

func memberOf(u dbgen.User) Member {
	status := StatusActive
	if u.DisabledAt.Valid {
		status = StatusDisabled
	}
	return Member{
		ID: uuidString(u.ID), Name: u.DisplayName, Color: u.AvatarColor, Role: u.Role, Status: status,
		Email: u.Email.String, HasPassword: u.PasswordHash.Valid,
	}
}

// ReadyForBootstrap reports whether no member exists yet, so a first-run admin invite makes sense.
func (p *PG) ReadyForBootstrap(ctx context.Context) (bool, error) {
	n, err := p.q.CountUsers(ctx)
	return n == 0, err
}

func (p *PG) CreateInvite(ctx context.Context, memberID string) (*Invite, error) {
	var member pgtype.UUID
	if memberID != "" {
		id, ok := parseUUID(memberID)
		if !ok {
			return nil, ErrNotFound
		}
		u, err := p.q.GetUser(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		}
		if u.DisabledAt.Valid {
			return nil, ErrDisabled
		}
		member = id
	}
	tok := token()
	exp := time.Now().Add(InviteTTL)
	err := p.q.CreateInvite(ctx, dbgen.CreateInviteParams{
		TokenHash: hashToken(tok), MemberID: member,
		ExpiresAt: pgtype.Timestamptz{Time: exp, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return &Invite{Token: tok, MemberID: memberID, ExpiresAt: exp}, nil
}

func (p *PG) Invite(ctx context.Context, tok string) (*Invite, *Member, error) {
	row, err := p.q.GetInvite(ctx, hashToken(tok))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	} else if err != nil {
		return nil, nil, err
	}
	inv := &Invite{Token: tok, MemberID: uuidString(row.MemberID), ExpiresAt: row.ExpiresAt.Time, Used: row.UsedBy.Valid}
	if !row.MemberID.Valid {
		return inv, nil, nil
	}
	u, err := p.q.GetUser(ctx, row.MemberID)
	if err != nil {
		return nil, nil, err
	}
	m := memberOf(u)
	return inv, &m, nil
}

func (p *PG) Accept(ctx context.Context, tok string, in AcceptInput) (*Member, string, *Challenge, error) {
	email, pwHash, err := prepareCredentials(in)
	if err != nil {
		return nil, "", nil, err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, "", nil, err
	}
	defer tx.Rollback(ctx)
	q := p.q.WithTx(tx)

	hash := hashToken(tok)
	inv, err := q.GetInviteForUpdate(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil, ErrNotFound
	} else if err != nil {
		return nil, "", nil, err
	}
	if inv.UsedBy.Valid {
		return nil, "", nil, ErrInviteUsed
	}
	if time.Now().After(inv.ExpiresAt.Time) {
		return nil, "", nil, ErrExpired
	}

	var u dbgen.User
	if inv.MemberID.Valid {
		u, err = q.GetUser(ctx, inv.MemberID)
		if err != nil {
			return nil, "", nil, err
		}
		if u.DisabledAt.Valid {
			return nil, "", nil, ErrDisabled
		}
		if err := q.SetUserProfile(ctx, dbgen.SetUserProfileParams{ID: u.ID, DisplayName: in.Name, AvatarColor: in.Color}); err != nil {
			return nil, "", nil, err
		}
		if err := q.ReplacePasskeys(ctx, u.ID); err != nil {
			return nil, "", nil, err
		}
		u.DisplayName, u.AvatarColor = in.Name, in.Color
	} else {
		// Serialise first-run so two bootstrap invites cannot both mint an admin.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7001)`); err != nil {
			return nil, "", nil, err
		}
		n, err := q.CountUsers(ctx)
		if err != nil {
			return nil, "", nil, err
		}
		role := RoleMember
		if n == 0 {
			role = RoleAdmin
		}
		u, err = q.CreateUser(ctx, dbgen.CreateUserParams{DisplayName: in.Name, Role: role, AvatarColor: in.Color})
		if err != nil {
			return nil, "", nil, err
		}
	}

	var ch *Challenge
	if pwHash != "" {
		if err := q.SetUserPassword(ctx, dbgen.SetUserPasswordParams{ID: u.ID, PasswordHash: pgtype.Text{String: pwHash, Valid: true}}); err != nil {
			return nil, "", nil, err
		}
		u.PasswordHash = pgtype.Text{String: pwHash, Valid: true}
	}
	if email != "" && email != u.Email.String {
		if other, err := q.GetUserByEmail(ctx, email); err == nil && other.ID != u.ID {
			return nil, "", nil, ErrEmailTaken
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, "", nil, err
		}
		c, err := p.insertChallenge(ctx, q, u.ID, PurposeVerify, email, u.DisplayName)
		if err != nil {
			return nil, "", nil, err
		}
		ch = &c
	}

	// The credential id is stored as-is until WebAuthn attestation lands (IMP-29).
	if err := q.AddPasskey(ctx, dbgen.AddPasskeyParams{
		CredentialID: []byte(in.CredentialID), UserID: u.ID, PublicKey: []byte{}, Transports: []string{},
	}); err != nil {
		return nil, "", nil, err
	}
	if err := q.MarkInviteUsed(ctx, dbgen.MarkInviteUsedParams{TokenHash: hash, UsedBy: u.ID}); err != nil {
		return nil, "", nil, err
	}
	sess := token()
	if err := q.CreateSession(ctx, dbgen.CreateSessionParams{
		ID: sess, UserID: u.ID, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(SessionTTL), Valid: true},
	}); err != nil {
		return nil, "", nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", nil, err
	}
	m := memberOf(u)
	return &m, sess, ch, nil
}

func (p *PG) Session(ctx context.Context, sess string) (*Member, error) {
	u, err := p.q.GetSessionUser(ctx, sess)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	m := memberOf(u)
	return &m, nil
}

func (p *PG) Members(ctx context.Context) ([]Member, error) {
	users, err := p.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Member, 0, len(users))
	for _, u := range users {
		out = append(out, memberOf(u))
	}
	return out, nil
}

func (p *PG) SetStatus(ctx context.Context, id, status string) error {
	uid, ok := parseUUID(id)
	if !ok {
		return ErrNotFound
	}
	if _, err := p.q.GetUser(ctx, uid); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	disabled := status == StatusDisabled
	if err := p.q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{ID: uid, Disabled: disabled}); err != nil {
		return err
	}
	if disabled {
		return p.q.DeleteUserSessions(ctx, uid)
	}
	return nil
}
