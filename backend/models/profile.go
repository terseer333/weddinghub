package models

import (
	"strings"
	"time"
)

// ProfileKeyAdmin is the profile key used by the wedding admin capability holder.
// Committee members use their committee member id, so each actor owns exactly one
// profile per wedding.
const ProfileKeyAdmin = "admin"

const (
	// MaxProfileDisplayNameLength bounds the self-service display name.
	MaxProfileDisplayNameLength = 80
	// MaxProfileAvatarBytes bounds the stored avatar. Avatars arrive from the browser
	// already resized to a small JPEG data URL, so anything larger is rejected rather
	// than stored.
	MaxProfileAvatarBytes = 512 * 1024
)

// Profile is the self-service identity of one actor in a wedding: the admin or a
// committee member. It is deliberately small — a display name and an avatar — while
// everything else about the actor (role, email, membership) stays on its own record.
type Profile struct {
	WeddingID   string    `json:"wedding_id,omitempty"`
	ID          string    `json:"id"`
	Role        Role      `json:"role"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// ValidProfileAvatar reports whether avatar is empty (no photo) or a bounded base64
// image data URL. Storing only data URLs keeps the API free of file uploads.
func ValidProfileAvatar(avatar string) bool {
	if avatar == "" {
		return true
	}
	if len(avatar) > MaxProfileAvatarBytes {
		return false
	}
	if !strings.HasPrefix(avatar, "data:image/") {
		return false
	}
	return strings.Contains(avatar, ";base64,")
}

// NormalizeProfileDisplayName trims a submitted display name and reports whether it
// is present and within bounds.
func NormalizeProfileDisplayName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxProfileDisplayNameLength {
		return name, false
	}
	return name, true
}
