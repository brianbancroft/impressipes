// Package uuidx converts between the API's string ids and pgx's UUID type.
package uuidx

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgtype"
)

// Parse reads a canonical UUID string.
func Parse(s string) (pgtype.UUID, bool) {
	var u pgtype.UUID
	if len(s) != 36 {
		return u, false
	}
	if err := u.Scan(s); err != nil {
		return u, false
	}
	return u, true
}

// String renders a UUID, or "" when it is null.
func String(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	h := hex.EncodeToString(u.Bytes[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// Ptr renders a UUID for an optional JSON field.
func Ptr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := String(u)
	return &s
}

// New returns a random (version 4) UUID.
func New() pgtype.UUID {
	var u pgtype.UUID
	if _, err := rand.Read(u.Bytes[:]); err != nil {
		panic(err)
	}
	u.Bytes[6] = u.Bytes[6]&0x0f | 0x40
	u.Bytes[8] = u.Bytes[8]&0x3f | 0x80
	u.Valid = true
	return u
}
