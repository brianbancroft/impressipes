package family

import "context"

// Directory is the household model the API runs on: members, invites and sessions.
// Store is the in-memory implementation; PG persists to Postgres.
type Directory interface {
	CreateInvite(ctx context.Context, memberID string) (*Invite, error)
	Invite(ctx context.Context, token string) (*Invite, *Member, error)
	// Accept redeems an invite. The Challenge, when non-nil, confirms the email the
	// member gave and must be mailed by the caller.
	Accept(ctx context.Context, token string, in AcceptInput) (*Member, string, *Challenge, error)
	Session(ctx context.Context, sess string) (*Member, error)
	Members(ctx context.Context) ([]Member, error)
	SetStatus(ctx context.Context, id, status string) error

	// PasswordLogin signs in with a confirmed email and a password.
	PasswordLogin(ctx context.Context, email, password string) (*Member, string, error)
	SetPassword(ctx context.Context, memberID, password string) error
	// StartLogin issues an emailed link + code for a confirmed address. It returns
	// nil when there is nothing to send, so callers answer identically either way.
	StartLogin(ctx context.Context, email string) (*Challenge, error)
	// StartVerify issues a challenge that confirms email for a signed-in member.
	StartVerify(ctx context.Context, memberID, email string) (*Challenge, error)
	CompleteLink(ctx context.Context, token string) (*Member, string, error)
	CompleteCode(ctx context.Context, email, code string) (*Member, string, error)

	// SignIn starts a session for a member who has already proven who they are
	// (a passkey assertion or an Apple identity token).
	SignIn(ctx context.Context, memberID string) (*Member, string, error)
	// MemberByEmail finds the member whose confirmed email is email.
	MemberByEmail(ctx context.Context, email string) (*Member, error)

	// Passkeys are WebAuthn credentials, which sync between a person's Apple devices.
	AddPasskey(ctx context.Context, memberID string, p Passkey) error
	Passkeys(ctx context.Context, memberID string) ([]Passkey, error)
	// PasskeyOwner finds a passkey and the member who registered it.
	PasskeyOwner(ctx context.Context, credentialID []byte) (*Passkey, *Member, error)
	TouchPasskey(ctx context.Context, credentialID []byte, signCount uint32, backedUp bool) error
	DeletePasskey(ctx context.Context, memberID string, credentialID []byte) error

	// Apple links one Apple ID (identified by its stable "sub") to a member.
	AppleLink(ctx context.Context, memberID, sub, email string) error
	AppleMember(ctx context.Context, sub string) (*Member, error)
	AppleIdentity(ctx context.Context, memberID string) (*AppleID, error)
	AppleUnlink(ctx context.Context, memberID string) error
}

var (
	_ Directory = (*Store)(nil)
	_ Directory = (*PG)(nil)
)
