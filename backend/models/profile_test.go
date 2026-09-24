package models

import (
	"strings"
	"testing"
)

func TestValidProfileAvatar(t *testing.T) {
	if !ValidProfileAvatar("") {
		t.Fatal("an empty avatar must be allowed so a photo can be removed")
	}
	if !ValidProfileAvatar("data:image/png;base64,iVBORw0KGgo=") {
		t.Fatal("a small image data URL should be accepted")
	}
	for name, avatar := range map[string]string{
		"not an image": "data:text/html;base64,PHNjcmlwdD4=",
		"remote url":   "https://example.test/photo.png",
		"missing data": "data:image/png,notbase64",
		"oversize":     "data:image/png;base64," + strings.Repeat("A", MaxProfileAvatarBytes),
	} {
		if ValidProfileAvatar(avatar) {
			t.Fatalf("%s avatar was accepted", name)
		}
	}
}

func TestNormalizeProfileDisplayName(t *testing.T) {
	name, ok := NormalizeProfileDisplayName("  Ada Lovelace  ")
	if !ok || name != "Ada Lovelace" {
		t.Fatalf("name = %q, ok = %v", name, ok)
	}
	if _, ok := NormalizeProfileDisplayName("   "); ok {
		t.Fatal("a blank display name was accepted")
	}
	if _, ok := NormalizeProfileDisplayName(strings.Repeat("x", MaxProfileDisplayNameLength+1)); ok {
		t.Fatal("an overlong display name was accepted")
	}
}
