package models

import (
	"strings"
	"time"
)

// User is a registered WeddingHub account. Identity is the normalized email;
// access requires the account password plus an emailed verification code.
// Password and lockout state never leave the server (json:"-").
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	PasswordHash    string    `json:"-"`
	PasswordAttempt int       `json:"-"` // consecutive failed password checks
	LockedUntil     time.Time `json:"-"` // brute-force lockout; zero when unlocked
}

// ValidPassword applies a deliberately conservative policy: long enough to
// resist guessing, and not purely one character class.
func ValidPassword(password string) bool {
	if len(password) < 10 || len(password) > 128 {
		return false
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

// LoginCode is a pending 6-digit verification code. Only the SHA-256 hash is
// retained, mirroring the capability-token handling used for invitations.
type LoginCode struct {
	CodeHash  string    `json:"-"`
	ExpiresAt time.Time `json:"-"`
	Attempts  int       `json:"-"`
}

// Session is an issued login session for one user. Only the token hash is stored.
type Session struct {
	UserID    string    `json:"-"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"-"`
}

// NormalizeEmail canonicalizes an address so " Alice@Example.com " and
// "alice@example.com" resolve to the same account.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidEmail applies a deliberately simple shape check; delivery is the real test.
func ValidEmail(email string) bool {
	email = NormalizeEmail(email)
	at := strings.Index(email, "@")
	if at <= 0 || at != strings.LastIndex(email, "@") {
		return false
	}
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")
	return len(domain) > 3 && dot > 0 && dot < len(domain)-1 && !strings.ContainsAny(domain, " ")
}
