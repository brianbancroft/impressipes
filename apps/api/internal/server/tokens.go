package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

// tokenPrefix marks a personal token, so a leaked one is easy to spot and revoke.
const tokenPrefix = "imp_"

type memberKey struct{}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

// NewToken mints a personal token for a member and returns it. Only its hash is
// stored, so this is the one time the token can be read.
func NewToken(ctx context.Context, pool *pgxpool.Pool, userID, name string) (string, error) {
	user, ok := uuidx.Parse(userID)
	if !ok {
		return "", fail(http.StatusBadRequest, "invalid user id")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := tokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	_, err := dbgen.New(pool).CreateAPIToken(ctx, dbgen.CreateAPITokenParams{
		UserID: user, Name: strings.TrimSpace(name), TokenHash: hashToken(tok),
	})
	return tok, err
}

// bearer resolves an Authorization header to the member and the client name the
// token was minted for. ok is false when the token is missing, unknown or revoked.
func (s *server) bearer(ctx context.Context, header string) (m *family.Member, client string, ok bool) {
	tok, found := strings.CutPrefix(header, "Bearer ")
	if !found || !strings.HasPrefix(tok, tokenPrefix) || s.q == nil {
		return nil, "", false
	}
	row, err := s.q.GetAPITokenUser(ctx, hashToken(tok))
	if err != nil {
		return nil, "", false
	}
	_ = s.q.TouchAPIToken(ctx, row.TokenID)
	status := family.StatusActive
	if row.DisabledAt.Valid {
		status = family.StatusDisabled
	}
	return &family.Member{
		ID: uuidx.String(row.ID), Name: row.DisplayName, Color: row.AvatarColor, Role: row.Role, Status: status,
	}, row.TokenName, true
}
