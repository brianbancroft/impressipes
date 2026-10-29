package family

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
)

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (p *PG) insertChallenge(ctx context.Context, q *dbgen.Queries, user pgtype.UUID, purpose, email, name string) (Challenge, error) {
	c, linkHash, codeHash := newChallenge(purpose, email, name)
	err := q.CreateLoginCode(ctx, dbgen.CreateLoginCodeParams{
		UserID: user, Purpose: purpose, Email: email, LinkHash: linkHash, CodeHash: codeHash,
		ExpiresAt: ts(time.Now().Add(LoginTTL)),
	})
	return c, err
}

func (p *PG) session(ctx context.Context, q *dbgen.Queries, user pgtype.UUID) (string, error) {
	sess := token()
	err := q.CreateSession(ctx, dbgen.CreateSessionParams{ID: sess, UserID: user, ExpiresAt: ts(time.Now().Add(SessionTTL))})
	return sess, err
}

func (p *PG) PasswordLogin(ctx context.Context, email, password string) (*Member, string, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, "", ErrBadCredentials
	}
	u, err := p.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		CheckPassword("", password)
		return nil, "", ErrBadCredentials
	} else if err != nil {
		return nil, "", err
	}
	if !CheckPassword(u.PasswordHash.String, password) {
		return nil, "", ErrBadCredentials
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

func (p *PG) SetPassword(ctx context.Context, memberID, password string) error {
	uid, ok := parseUUID(memberID)
	if !ok {
		return ErrNotFound
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return p.q.SetUserPassword(ctx, dbgen.SetUserPasswordParams{ID: uid, PasswordHash: pgtype.Text{String: hash, Valid: true}})
}

// StartLogin issues a sign-in challenge for a confirmed address. It returns nil
// when there is nothing to send (unknown or disabled member, or too many recent
// requests) so the caller can answer the same way every time.
func (p *PG) StartLogin(ctx context.Context, email string) (*Challenge, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, nil
	}
	u, err := p.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.DisabledAt.Valid) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	n, err := p.q.CountRecentLoginCodes(ctx, dbgen.CountRecentLoginCodesParams{Email: email, CreatedAt: ts(time.Now().Add(-time.Hour))})
	if err != nil || n >= MaxChallengesPerHour {
		return nil, err
	}
	c, err := p.insertChallenge(ctx, p.q, u.ID, PurposeLogin, email, u.DisplayName)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// StartVerify issues a challenge that confirms email for a signed-in member.
func (p *PG) StartVerify(ctx context.Context, memberID, email string) (*Challenge, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	uid, ok := parseUUID(memberID)
	if !ok {
		return nil, ErrNotFound
	}
	u, err := p.q.GetUser(ctx, uid)
	if err != nil {
		return nil, ErrNotFound
	}
	if other, err := p.q.GetUserByEmail(ctx, email); err == nil && other.ID != uid {
		return nil, ErrEmailTaken
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	n, err := p.q.CountRecentLoginCodes(ctx, dbgen.CountRecentLoginCodesParams{Email: email, CreatedAt: ts(time.Now().Add(-time.Hour))})
	if err != nil {
		return nil, err
	}
	if n >= MaxChallengesPerHour {
		return nil, ErrTooMany
	}
	c, err := p.insertChallenge(ctx, p.q, uid, PurposeVerify, email, u.DisplayName)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CompleteLink redeems an emailed link token.
func (p *PG) CompleteLink(ctx context.Context, tok string) (*Member, string, error) {
	return p.complete(ctx, func(q *dbgen.Queries) (dbgen.LoginCode, error) {
		return q.GetLoginCodeByLinkForUpdate(ctx, hashToken(tok))
	}, "")
}

// CompleteCode redeems the six-digit code from the latest challenge sent to email.
func (p *PG) CompleteCode(ctx context.Context, email, code string) (*Member, string, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, "", ErrInvalidLogin
	}
	return p.complete(ctx, func(q *dbgen.Queries) (dbgen.LoginCode, error) {
		return q.GetLatestLoginCodeForUpdate(ctx, email)
	}, hashCode(email, code))
}

// complete locks a challenge, checks it (and the code, when codeHash is set) and signs the member in.
func (p *PG) complete(ctx context.Context, find func(*dbgen.Queries) (dbgen.LoginCode, error), codeHash string) (*Member, string, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	q := p.q.WithTx(tx)

	lc, err := find(q)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrInvalidLogin
	} else if err != nil {
		return nil, "", err
	}
	if lc.UsedAt.Valid || time.Now().After(lc.ExpiresAt.Time) || lc.Attempts >= MaxCodeAttempts {
		return nil, "", ErrInvalidLogin
	}
	if codeHash != "" && !sameHash(lc.CodeHash, codeHash) {
		// Count the miss even though the caller gets an error.
		if err := q.BumpLoginCodeAttempts(ctx, lc.ID); err != nil {
			return nil, "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, "", err
		}
		return nil, "", ErrInvalidLogin
	}

	u, err := q.GetUser(ctx, lc.UserID)
	if err != nil {
		return nil, "", err
	}
	if u.DisabledAt.Valid {
		return nil, "", ErrDisabled
	}
	if lc.Purpose == PurposeVerify {
		if err := q.SetUserEmail(ctx, dbgen.SetUserEmailParams{ID: u.ID, Email: pgtype.Text{String: lc.Email, Valid: true}}); err != nil {
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23505" {
				return nil, "", ErrEmailTaken
			}
			return nil, "", err
		}
		u.Email = pgtype.Text{String: lc.Email, Valid: true}
	}
	if err := q.UseLoginCode(ctx, lc.ID); err != nil {
		return nil, "", err
	}
	sess, err := p.session(ctx, q, u.ID)
	if err != nil {
		return nil, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	m := memberOf(u)
	return &m, sess, nil
}
