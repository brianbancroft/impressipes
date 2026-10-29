package family

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	stdmail "net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	// LoginTTL is how long an emailed link and code stay valid.
	LoginTTL = 15 * time.Minute
	// MaxCodeAttempts is how many wrong codes one challenge tolerates.
	MaxCodeAttempts = 5
	// MaxChallengesPerHour caps how many emails one address can be sent per hour.
	MaxChallengesPerHour = 5
	// MinPasswordLen is the only password rule: length beats composition.
	MinPasswordLen = 10

	PurposeLogin  = "login"
	PurposeVerify = "verify"
)

var (
	ErrBadCredentials = errors.New("invalid email or password")
	ErrInvalidLogin   = errors.New("invalid or expired link or code")
	ErrEmailTaken     = errors.New("email already in use")
	ErrInvalidEmail   = errors.New("invalid email")
	ErrWeakPassword   = fmt.Errorf("password must be at least %d characters", MinPasswordLen)
	ErrTooMany        = errors.New("too many requests")
	// ErrCredentialsRequired is returned when a new member joins without an email and password.
	ErrCredentialsRequired = errors.New("an email and a password are required to join")
)

// Challenge is an emailed link + code. The caller mails it; only its hashes are stored.
type Challenge struct {
	Purpose string
	Email   string
	Name    string
	Token   string // the link token
	Code    string // six digits
}

// AcceptInput is what an invite is redeemed with. Email and Password are optional;
// an email is confirmed afterwards through the Challenge that Accept returns.
type AcceptInput struct {
	Name, Color, CredentialID string
	Email, Password           string
}

// NormalizeEmail lower-cases and validates an address.
func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := stdmail.ParseAddress(s)
	if err != nil || a.Address != s || !strings.Contains(s[strings.LastIndex(s, "@"):], ".") || len(s) > 254 {
		return "", ErrInvalidEmail
	}
	return s, nil
}

func newChallenge(purpose, email, name string) (c Challenge, linkHash, codeHash string) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	c = Challenge{Purpose: purpose, Email: email, Name: name, Token: token(), Code: fmt.Sprintf("%06d", n.Int64())}
	return c, hashToken(c.Token), hashCode(email, c.Code)
}

func hashCode(email, code string) string { return hashToken("code:" + email + ":" + code) }

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func sameHash(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

const (
	argonTime, argonMem, argonThreads, argonKey = 2, 19 * 1024, 1, 32
)

// HashPassword returns an argon2id hash in a self-describing form.
func HashPassword(pw string) (string, error) {
	if len([]rune(pw)) < MinPasswordLen {
		return "", ErrWeakPassword
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(pw), salt, argonTime, argonMem, argonThreads, argonKey)
	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s", argonTime, argonMem, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum)), nil
}

// CheckPassword reports whether pw matches hash. An empty hash still burns the
// same time, so sign-in does not reveal which emails have a password.
func CheckPassword(hash, pw string) bool {
	var t, m uint32
	var p uint8
	var saltB64, sumB64 string
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		argon2.IDKey([]byte(pw), make([]byte, 16), argonTime, argonMem, argonThreads, argonKey)
		return false
	}
	if _, err := fmt.Sscanf(strings.Join(parts[1:4], " "), "%d %d %d", &t, &m, &p); err != nil {
		return false
	}
	saltB64, sumB64 = parts[4], parts[5]
	salt, err1 := base64.RawStdEncoding.DecodeString(saltB64)
	want, err2 := base64.RawStdEncoding.DecodeString(sumB64)
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// prepareCredentials validates and hashes the optional email and password of an
// invite redemption before any transaction is opened.
func prepareCredentials(in AcceptInput) (email, pwHash string, err error) {
	if in.Email != "" {
		if email, err = NormalizeEmail(in.Email); err != nil {
			return "", "", err
		}
	}
	if in.Password != "" {
		if pwHash, err = HashPassword(in.Password); err != nil {
			return "", "", err
		}
	}
	return email, pwHash, nil
}
