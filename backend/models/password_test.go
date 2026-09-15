package models

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery 9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected encoding: %q", hash)
	}
	match, err := VerifyPassword(hash, "correct horse battery 9")
	if err != nil || !match {
		t.Fatalf("correct password rejected: match=%v err=%v", match, err)
	}
	match, err = VerifyPassword(hash, "correct horse battery 8")
	if err != nil || match {
		t.Fatalf("wrong password accepted: match=%v err=%v", match, err)
	}
	// The same password must hash to a different string each time (random salt).
	again, err := HashPassword("correct horse battery 9")
	if err != nil {
		t.Fatal(err)
	}
	if again == hash {
		t.Fatal("salt reuse: two hashes of one password are identical")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	for _, hash := range []string{"", "plaintext", "$argon2id$v=19$broken", "$argon2id$v=99$m=19456,t=2,p=1$AAAA$AAAA"} {
		if _, err := VerifyPassword(hash, "whatever-password-1"); err == nil {
			t.Fatalf("malformed hash %q accepted", hash)
		}
	}
}

func TestValidPasswordPolicy(t *testing.T) {
	invalid := []string{"", "short1", "only-letters-here", "123456789012", strings.Repeat("a1", 65)}
	for _, password := range invalid {
		if ValidPassword(password) {
			t.Fatalf("weak password accepted: %q", password)
		}
	}
	if !ValidPassword("long-enough-1") || !ValidPassword(strings.Repeat("a1", 64)) {
		t.Fatal("strong password rejected")
	}
}
