package models

import (
	"encoding/json"
	"testing"
)

func TestOpaqueTokenAndHash(t *testing.T) {
	token, hash, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 {
		t.Fatalf("token length = %d", len(token))
	}
	if len(hash) != 64 || HashToken(token) != hash {
		t.Fatal("invalid token hash")
	}
	encoded, err := json.Marshal(Invitation{TokenHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || contains(string(encoded), hash) {
		t.Fatal("token hash leaked through JSON")
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
