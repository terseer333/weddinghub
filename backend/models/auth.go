package models

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Password hashing uses PBKDF2-HMAC-SHA256 implemented on the standard library so the
// API keeps zero external dependencies. The stored form is self-describing:
//
//	pbkdf2-sha256$<iterations>$<salt-base64>$<derived-key-base64>
//
// Verification reads the iteration count from the record, so PasswordIterations can be
// raised later without invalidating existing credentials.
const (
	passwordAlgorithm = "pbkdf2-sha256"
	passwordSaltBytes = 16
	passwordKeyBytes  = 32
	// minPasswordKeyBytes rejects implausibly short derived keys in stored records.
	minPasswordKeyBytes = 16

	// MinPasswordLength is the shortest password the API accepts.
	MinPasswordLength = 8
)

// PasswordIterations is the PBKDF2 work factor used for newly hashed passwords.
var PasswordIterations = 210000

var (
	// ErrInvalidPasswordHash reports a malformed stored hash, which is never treated as a match.
	ErrInvalidPasswordHash = errors.New("invalid password hash")
	// ErrPasswordMismatch reports that a password does not match its stored hash.
	ErrPasswordMismatch = errors.New("password does not match")
)

// HashPassword derives a fresh salted hash for storage.
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := pbkdf2SHA256([]byte(password), salt, PasswordIterations, passwordKeyBytes)
	return fmt.Sprintf("%s$%d$%s$%s", passwordAlgorithm, PasswordIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the stored hash. The comparison is
// constant time and a malformed record is rejected rather than treated as a match.
func VerifyPassword(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return ErrInvalidPasswordHash
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return ErrInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return ErrInvalidPasswordHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) < minPasswordKeyBytes {
		return ErrInvalidPasswordHash
	}
	got := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

// pbkdf2SHA256 implements PBKDF2 (RFC 2898) with HMAC-SHA-256.
func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLength := prf.Size()
	blocks := (keyLength + hashLength - 1) / hashLength
	derived := make([]byte, 0, blocks*hashLength)
	counter := make([]byte, 4)
	for block := 1; block <= blocks; block++ {
		prf.Reset()
		prf.Write(salt)
		counter[0] = byte(block >> 24)
		counter[1] = byte(block >> 16)
		counter[2] = byte(block >> 8)
		counter[3] = byte(block)
		prf.Write(counter)
		u := prf.Sum(nil)
		accumulator := make([]byte, len(u))
		copy(accumulator, u)
		for i := 1; i < iterations; i++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(nil)
			for j := range accumulator {
				accumulator[j] ^= u[j]
			}
		}
		derived = append(derived, accumulator...)
	}
	return derived[:keyLength]
}

// NormalizeEmail lowercases and trims an address so account lookups are case-insensitive.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Session is an authenticated browser session. Only the token hash is persisted, so a
// repository dump cannot be replayed as a live session.
type Session struct {
	TokenHash string    `json:"-"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
