package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// PublicationStatus controls whether wedding content is editable-only, guest-visible, or suppressed.
type PublicationStatus string

const (
	StatusDraft     PublicationStatus = "draft"
	StatusPublished PublicationStatus = "published"
	StatusHidden    PublicationStatus = "hidden"
)

func (s PublicationStatus) Valid() bool {
	return s == StatusDraft || s == StatusPublished || s == StatusHidden
}

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
	RoleGuest Role = "guest"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Admin struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

type Guest struct {
	ID           string `json:"id"`
	InvitationID string `json:"invitation_id"`
	Name         string `json:"name"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
}

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationDeclined InvitationStatus = "declined"
)

type Invitation struct {
	ID           string           `json:"id"`
	GuestName    string           `json:"guest_name"`
	GuestEmail   string           `json:"guest_email,omitempty"`
	MaxPartySize int              `json:"max_party_size"`
	Status       InvitationStatus `json:"status"`
	TokenHash    string           `json:"-"`
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	RespondedAt  *time.Time       `json:"responded_at,omitempty"`
}

type Event struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`
	Venue       string            `json:"venue,omitempty"`
	Address     string            `json:"address,omitempty"`
	Status      PublicationStatus `json:"status"`
}

type Photo struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	AltText   string            `json:"alt_text,omitempty"`
	Caption   string            `json:"caption,omitempty"`
	SortOrder int               `json:"sort_order"`
	Status    PublicationStatus `json:"status"`
}

type StorySection struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	PhotoURL  string            `json:"photo_url,omitempty"`
	SortOrder int               `json:"sort_order"`
	Status    PublicationStatus `json:"status"`
}

type Announcement struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	PublishedAt *time.Time        `json:"published_at,omitempty"`
	Status      PublicationStatus `json:"status"`
}

type RSVPStatus string

const (
	RSVPAttending    RSVPStatus = "attending"
	RSVPNotAttending RSVPStatus = "not_attending"
	RSVPMaybe        RSVPStatus = "maybe"
)

type RSVP struct {
	InvitationID string     `json:"invitation_id"`
	Status       RSVPStatus `json:"status"`
	PartySize    int        `json:"party_size"`
	DietaryNotes string     `json:"dietary_notes,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type GuestMessage struct {
	ID           string    `json:"id"`
	WeddingID    string    `json:"wedding_id"`
	InvitationID string    `json:"invitation_id"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

// Wedding is the consistency boundary for its administrators, guests, content, and responses.
type Wedding struct {
	ID            string            `json:"id"`
	Slug          string            `json:"slug"`
	Title         string            `json:"title"`
	PartnerOne    string            `json:"partner_one"`
	PartnerTwo    string            `json:"partner_two"`
	Date          *time.Time        `json:"date,omitempty"`
	Status        PublicationStatus `json:"status"`
	Venue         string            `json:"venue,omitempty"`
	Address       string            `json:"address,omitempty"`
	City          string            `json:"city,omitempty"`
	State         string            `json:"state,omitempty"`
	Country       string            `json:"country,omitempty"`
	Message       string            `json:"message,omitempty"`
	Verse         string            `json:"verse,omitempty"`
	DressCode     string            `json:"dress_code,omitempty"`
	HeroImage     string            `json:"hero_image,omitempty"`
	TemplateID    string            `json:"template_id,omitempty"`
	Admins        []Admin           `json:"admins"`
	Guests        []Guest           `json:"guests"`
	Invitations   []Invitation      `json:"invitations"`
	Events        []Event           `json:"events"`
	Photos        []Photo           `json:"photos"`
	StorySections []StorySection    `json:"story_sections"`
	Announcements []Announcement    `json:"announcements"`
	RSVPs         []RSVP            `json:"rsvps"`
	GuestMessages []GuestMessage    `json:"guest_messages"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// NewOpaqueToken returns a 256-bit, URL-safe capability token and its SHA-256 hash.
// Only the hash should be persisted; the raw token is returned to the invitee once.
func NewOpaqueToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashToken(token), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
