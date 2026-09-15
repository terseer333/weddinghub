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

// Audience decides whether published content reaches guests or stays inside the planning committee.
// An empty value is treated as AudiencePublic so records written before audiences existed stay guest-visible.
type Audience string

const (
	AudiencePublic    Audience = "public"
	AudienceCommittee Audience = "committee"
)

func (a Audience) Normalized() Audience {
	if a == AudienceCommittee {
		return AudienceCommittee
	}
	return AudiencePublic
}

func (a Audience) Valid() bool {
	return a == "" || a == AudiencePublic || a == AudienceCommittee
}

// GuestVisible reports whether content with this audience may appear in a guest projection.
func (a Audience) GuestVisible() bool {
	return a.Normalized() == AudiencePublic
}

type Role string

const (
	RoleOwner           Role = "owner"
	RoleAdmin           Role = "admin"
	RoleCommitteeMember Role = "committee_member"
	RoleGuest           Role = "guest"
)

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
	InvitationExpired  InvitationStatus = "expired"
)

// InvitationType separates the two invitation experiences a wedding can issue.
// An empty value is treated as InvitationGuest for records written before committee invitations existed.
type InvitationType string

const (
	InvitationGuest     InvitationType = "guest"
	InvitationCommittee InvitationType = "committee"
)

func (t InvitationType) Normalized() InvitationType {
	if t == InvitationCommittee {
		return InvitationCommittee
	}
	return InvitationGuest
}

func (t InvitationType) Valid() bool {
	return t == "" || t == InvitationGuest || t == InvitationCommittee
}

// Role maps an invitation type onto the wedding-scoped role it grants on acceptance.
func (t InvitationType) Role() Role {
	if t.Normalized() == InvitationCommittee {
		return RoleCommitteeMember
	}
	return RoleGuest
}

type Invitation struct {
	ID             string           `json:"id"`
	Type           InvitationType   `json:"type"`
	GuestName      string           `json:"guest_name"`
	GuestEmail     string           `json:"guest_email,omitempty"`
	GuestPhone     string           `json:"guest_phone,omitempty"`
	CommitteeTitle string           `json:"committee_title,omitempty"`
	MaxPartySize   int              `json:"max_party_size"`
	Status         InvitationStatus `json:"status"`
	TokenHash      string           `json:"-"`
	ExpiresAt      *time.Time       `json:"expires_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	RespondedAt    *time.Time       `json:"responded_at,omitempty"`
}

// CommitteeMember is created when a committee invitation is accepted. Membership is
// wedding-scoped: the same person can be a committee member for one wedding and a guest for another.
type CommitteeMember struct {
	ID           string    `json:"id"`
	InvitationID string    `json:"invitation_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	Title        string    `json:"title,omitempty"`
	RoleID       string    `json:"role_id,omitempty"`
	JoinedAt     time.Time `json:"joined_at"`
}

// CommitteeMessage is private planning conversation. It is never included in a guest projection.
type CommitteeMessage struct {
	ID         string    `json:"id"`
	WeddingID  string    `json:"wedding_id"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	AuthorRole Role      `json:"author_role"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type TaskStatus string

const (
	TaskTodo       TaskStatus = "todo"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	return s == TaskTodo || s == TaskInProgress || s == TaskDone
}

// PlanningTask is committee-only planning work. It is never included in a guest projection.
type PlanningTask struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Details    string     `json:"details,omitempty"`
	AssignedTo string     `json:"assigned_to,omitempty"`
	DueOn      string     `json:"due_on,omitempty"`
	Status     TaskStatus `json:"status"`
	CreatedBy  string     `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
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
	Audience    Audience          `json:"audience,omitempty"`
	PublishedAt *time.Time        `json:"published_at,omitempty"`
	Status      PublicationStatus `json:"status"`
	AuthorName  string            `json:"author_name,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
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

type CardFonts struct {
	Couple  string `json:"couple"`
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type CardColors struct {
	Background string `json:"background"`
	Text       string `json:"text"`
	Accent     string `json:"accent"`
	Border     string `json:"border,omitempty"`
	Secondary  string `json:"secondary,omitempty"`
}

type CardDecorations struct {
	FloralStyle string `json:"floral_style,omitempty"`
	BorderStyle string `json:"border_style,omitempty"`
	Layout      string `json:"layout,omitempty"`
	FrameGlow   bool   `json:"frame_glow,omitempty"`
	DatePill    bool   `json:"date_pill,omitempty"`
	Background  string `json:"background,omitempty"`
}

type CardConfig struct {
	TemplateID   string            `json:"template_id"`
	Fonts        CardFonts         `json:"fonts"`
	Colors       CardColors        `json:"colors"`
	Decorations  CardDecorations   `json:"decorations"`
	CustomStyles map[string]string `json:"custom_styles,omitempty"`
}

type CommitteeRole struct {
	ID          string    `json:"id"`
	WeddingID   string    `json:"wedding_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsCustom    bool      `json:"is_custom,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type Template struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Category      string          `json:"category"`
	CategoryLabel string          `json:"categoryLabel,omitempty"`
	Description   string          `json:"description,omitempty"`
	Fonts         CardFonts       `json:"fonts"`
	Colors        CardColors      `json:"colors"`
	Decorations   CardDecorations `json:"decorations"`
	Layout        string          `json:"layout,omitempty"`
	Background    map[string]any  `json:"background,omitempty"`
	Premium       bool            `json:"premium,omitempty"`
}

type FontItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Family   string `json:"family"`
}

// Wedding is the consistency boundary for its administrators, guests, committee, content, and responses.
type Wedding struct {
	ID         string            `json:"id"`
	Slug       string            `json:"slug"`
	Title      string            `json:"title"`
	PartnerOne string            `json:"partner_one"`
	PartnerTwo string            `json:"partner_two"`
	Date       *time.Time        `json:"date,omitempty"`
	Status     PublicationStatus `json:"status"`
	Venue      string            `json:"venue,omitempty"`
	Address    string            `json:"address,omitempty"`
	City       string            `json:"city,omitempty"`
	State      string            `json:"state,omitempty"`
	Country    string            `json:"country,omitempty"`
	Message    string            `json:"message,omitempty"`
	Verse      string            `json:"verse,omitempty"`
	DressCode  string            `json:"dress_code,omitempty"`
	HeroImage  string            `json:"hero_image,omitempty"`
	TemplateID string            `json:"template_id,omitempty"`
	CardConfig *CardConfig       `json:"card_config,omitempty"`
	// AdminTokenHash authenticates wedding administration. Only the SHA-256 hash is retained
	// and it is never serialized; the raw token is returned once when the wedding is created.
	AdminTokenHash   string             `json:"-"`
	Admins           []Admin            `json:"admins"`
	Guests           []Guest            `json:"guests"`
	CommitteeMembers []CommitteeMember  `json:"committee_members"`
	CommitteeRoles   []CommitteeRole    `json:"committee_roles,omitempty"`
	Invitations      []Invitation       `json:"invitations"`
	Events           []Event            `json:"events"`
	Photos           []Photo            `json:"photos"`
	StorySections    []StorySection     `json:"story_sections"`
	Announcements    []Announcement     `json:"announcements"`
	PlanningTasks    []PlanningTask     `json:"planning_tasks"`
	CommitteeChat    []CommitteeMessage `json:"committee_chat"`
	RSVPs            []RSVP             `json:"rsvps"`
	GuestMessages    []GuestMessage     `json:"guest_messages"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
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
