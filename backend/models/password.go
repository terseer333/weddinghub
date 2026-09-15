package models

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters follow the OWASP baseline (19 MiB, 2 passes, 1 thread).
// Memory cost dominates: raising it slows each guess far more than it slows a
// single legitimate login on a lightly loaded server.
const (
	argonMemory  = 19 * 1024 // KiB
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

// ErrInvalidHashFormat is returned when a stored password hash cannot be parsed.
var ErrInvalidHashFormat = errors.New("stored password hash is malformed")

// HashPassword derives an argon2id hash encoded in PHC form:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>. Only this string is persisted.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches a stored PHC hash. The
// comparison is constant-time over the derived keys.
func VerifyPassword(hash, password string) (bool, error) {
	salt, key, params, err := decodePasswordHash(hash)
	if err != nil {
		return false, err
	}
	derived := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(derived, key) == 1, nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func decodePasswordHash(hash string) ([]byte, []byte, argonParams, error) {
	parts := strings.Split(hash, "$")
	// Split on "$argon2id$..." leaves an empty first element.
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, argonParams{}, ErrInvalidHashFormat
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, argonParams{}, ErrInvalidHashFormat
	}
	params := argonParams{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.time, &params.threads); err != nil {
		return nil, nil, argonParams{}, ErrInvalidHashFormat
	}
	if params.memory == 0 || params.time == 0 || params.threads == 0 {
		return nil, nil, argonParams{}, ErrInvalidHashFormat
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return nil, nil, argonParams{}, ErrInvalidHashFormat
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return nil, nil, argonParams{}, ErrInvalidHashFormat
	}
	return salt, key, params, nil
}

// dummyHash is a real argon2id hash of an unknowable random value. Verifying a
// supplied password against it lets login report the same error, at roughly the
// same cost, whether or not the email belongs to an account.
var dummyHash = func() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	hash, err := HashPassword(base64.RawURLEncoding.EncodeToString(buf))
	if err != nil {
		panic("could not prepare password comparison: " + err.Error())
	}
	return hash
}()

// DummyPasswordVerify burns the same argon2id work as a real verification. It
// never returns a match; the result of the comparison is discarded.
func DummyPasswordVerify(password string) {
	ok, err := VerifyPassword(dummyHash, password)
	_ = ok
	_ = err
}
