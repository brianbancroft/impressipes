package family

import (
	"context"
	"time"
)

func (s *Store) byEmail(email string) *Member {
	for _, m := range s.members {
		if m.Email == email {
			return m
		}
	}
	return nil
}

func (s *Store) newSession(m *Member) string {
	sess := token()
	s.sessions[sess] = m.ID
	return sess
}

func (s *Store) addChallenge(m *Member, purpose, email string) Challenge {
	c, linkHash, codeHash := newChallenge(purpose, email, m.Name)
	s.challenges[linkHash] = &challenge{
		userID: m.ID, purpose: purpose, email: email, codeHash: codeHash,
		expires: s.now().Add(LoginTTL), created: s.now(),
	}
	return c
}

func (s *Store) recentChallenges(email string) int {
	n := 0
	for _, c := range s.challenges {
		if c.email == email && c.created.After(s.now().Add(-time.Hour)) {
			n++
		}
	}
	return n
}

func (s *Store) PasswordLogin(_ context.Context, email, password string) (*Member, string, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, "", ErrBadCredentials
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.byEmail(email)
	if m == nil {
		CheckPassword("", password)
		return nil, "", ErrBadCredentials
	}
	if !CheckPassword(s.passwords[m.ID], password) {
		return nil, "", ErrBadCredentials
	}
	if m.Status == StatusDisabled {
		return nil, "", ErrDisabled
	}
	cp := *m
	return &cp, s.newSession(m), nil
}

func (s *Store) SetPassword(_ context.Context, memberID, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[memberID]
	if !ok {
		return ErrNotFound
	}
	s.passwords[m.ID], m.HasPassword = hash, true
	return nil
}

func (s *Store) StartLogin(_ context.Context, email string) (*Challenge, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.byEmail(email)
	if m == nil || m.Status == StatusDisabled || s.recentChallenges(email) >= MaxChallengesPerHour {
		return nil, nil
	}
	c := s.addChallenge(m, PurposeLogin, email)
	return &c, nil
}

func (s *Store) StartVerify(_ context.Context, memberID, email string) (*Challenge, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[memberID]
	if !ok {
		return nil, ErrNotFound
	}
	if other := s.byEmail(email); other != nil && other.ID != m.ID {
		return nil, ErrEmailTaken
	}
	if s.recentChallenges(email) >= MaxChallengesPerHour {
		return nil, ErrTooMany
	}
	c := s.addChallenge(m, PurposeVerify, email)
	return &c, nil
}

func (s *Store) CompleteLink(_ context.Context, tok string) (*Member, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.complete(s.challenges[hashToken(tok)], "")
}

func (s *Store) CompleteCode(_ context.Context, email, code string) (*Member, string, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, "", ErrInvalidLogin
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *challenge
	for _, c := range s.challenges {
		if c.email == email && !c.used && s.now().Before(c.expires) && c.attempts < MaxCodeAttempts &&
			(latest == nil || c.created.After(latest.created)) {
			latest = c
		}
	}
	return s.complete(latest, hashCode(email, code))
}

func (s *Store) complete(c *challenge, codeHash string) (*Member, string, error) {
	if c == nil || c.used || s.now().After(c.expires) || c.attempts >= MaxCodeAttempts {
		return nil, "", ErrInvalidLogin
	}
	if codeHash != "" && !sameHash(c.codeHash, codeHash) {
		c.attempts++
		return nil, "", ErrInvalidLogin
	}
	m := s.members[c.userID]
	if m.Status == StatusDisabled {
		return nil, "", ErrDisabled
	}
	if c.purpose == PurposeVerify {
		if other := s.byEmail(c.email); other != nil && other.ID != m.ID {
			return nil, "", ErrEmailTaken
		}
		m.Email = c.email
	}
	c.used = true
	cp := *m
	return &cp, s.newSession(m), nil
}
