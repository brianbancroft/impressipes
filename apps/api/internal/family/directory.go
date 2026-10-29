package family

import "context"

// Directory is the household model the API runs on: members, invites and sessions.
// Store is the in-memory implementation; PG persists to Postgres.
type Directory interface {
	CreateInvite(ctx context.Context, memberID string) (*Invite, error)
	Invite(ctx context.Context, token string) (*Invite, *Member, error)
	Accept(ctx context.Context, token, name, color, credentialID string) (*Member, string, error)
	Session(ctx context.Context, sess string) (*Member, error)
	Members(ctx context.Context) ([]Member, error)
	SetStatus(ctx context.Context, id, status string) error
}

var (
	_ Directory = (*Store)(nil)
	_ Directory = (*PG)(nil)
)
