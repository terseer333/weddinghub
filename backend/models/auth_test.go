package models

import (
	"encoding/base64"
	"strings"
	"testing"
)

// useFastHashing keeps the test suite quick while leaving the production work factor alone.
func useFastHashing(t *testing.T) {
	t.Helper()
	previous := PasswordIterations
	PasswordIterations = 1200
	t.Cleanup(func() { PasswordIterations = previous })
}

func TestPasswordHashingRoundTrip(t *testing.T) {
	useFastHashing(t)

	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "pbkdf2-sha256$1200$") {
		t.Fatalf("unexpected encoding: %q", hash)
	}
	if strings.Contains(hash, "correct horse battery staple") {
		t.Fatalf("stored hash contains the plaintext password: %q", hash)
	}
	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("verifying the right password failed: %v", err)
	}
	if err := VerifyPassword(hash, "wrong password"); err != ErrPasswordMismatch {
		t.Fatalf("wrong password error = %v", err)
	}

	second, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if second == hash {
		t.Fatal("equal passwords must not produce equal hashes (salt must be random)")
	}
	if err := VerifyPassword(second, "correct horse battery staple"); err != nil {
		t.Fatalf("verifying the second hash failed: %v", err)
	}
}

func TestVerifyPasswordRejectsMalformedRecords(t *testing.T) {
	useFastHashing(t)
	valid, err := HashPassword("supersecret")
	if err != nil {
		t.Fatal(err)
	}
	for name, encoded := range map[string]string{
		"empty":           "",
		"plaintext":       "supersecret",
		"unknown scheme":  "bcrypt$1200$c2FsdA$a2V5",
		"zero iterations": "pbkdf2-sha256$0$c2FsdA$a2V5",
		"missing fields":  "pbkdf2-sha256$1200",
		"bad salt base64": "pbkdf2-sha256$1200$!!!$a2V5",
		"bad key base64":  "pbkdf2-sha256$1200$c2FsdA$!!!",
		"empty salt":      "pbkdf2-sha256$1200$$a2V5",
		"short key":       "pbkdf2-sha256$1200$c2FsdA$a2V5",
		"wrong key value": wrongKeyRecord(valid),
	} {
		t.Run(name, func(t *testing.T) {
			if err := VerifyPassword(encoded, "supersecret"); err == nil {
				t.Fatalf("malformed hash %q was accepted", encoded)
			}
		})
	}
}

func TestVerifyPasswordHonoursStoredIterationCount(t *testing.T) {
	useFastHashing(t)
	hash, err := HashPassword("supersecret")
	if err != nil {
		t.Fatal(err)
	}
	// Raising the work factor for new hashes must not invalidate existing records.
	PasswordIterations = 4000
	if err := VerifyPassword(hash, "supersecret"); err != nil {
		t.Fatalf("existing hash stopped verifying after a work-factor change: %v", err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	for input, want := range map[string]string{
		"  Admin@Example.COM ": "admin@example.com",
		"ADA@EXAMPLE.COM":      "ada@example.com",
		"":                     "",
	} {
		if got := NormalizeEmail(input); got != want {
			t.Fatalf("NormalizeEmail(%q) = %q, want %q", input, got, want)
		}
	}
}

// wrongKeyRecord keeps the encoding valid but swaps the derived key for an unrelated
// value of the same length, which must not verify.
func wrongKeyRecord(encoded string) string {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return "pbkdf2-sha256$1200$c2FsdA$a2V5"
	}
	replacement := base64.RawStdEncoding.EncodeToString([]byte("an-unrelated-derived-key-value!!!"))
	return parts[0] + "$" + parts[1] + "$" + parts[2] + "$" + replacement
}
