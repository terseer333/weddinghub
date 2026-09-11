package models

import (
	"time"
)

// CommitteeMember represents a wedding planning committee member
type CommitteeMember struct {
	ID                 string    `json:"id"`
	WeddingID          string    `json:"wedding_id"`
	Name               string    `json:"name"`
	Email              string    `json:"email"`
	Phone              string    `json:"phone,omitempty"`
	Role               string    `json:"role"` // Best Man, Maid of Honor, Bridesmaid, etc.
	Status             string    `json:"status"` // pending, accepted, declined
	Token              string    `json:"token,omitempty"` // Secure invitation token
	Permissions        []string  `json:"permissions"` // edit_wedding, manage_events, etc.
	InvitationStatus   string    `json:"invitation_status"` // sent, opened, accepted
	InvitedAt          time.Time `json:"invited_at"`
	AcceptedAt         *time.Time `json:"accepted_at,omitempty"`
	DeclinedAt         *time.Time `json:"declined_at,omitempty"`
	JoinedAt           *time.Time `json:"joined_at,omitempty"`
	LastActiveAt       *time.Time `json:"last_active_at,omitempty"`
}

// CommitteeTask represents a wedding planning task
type CommitteeTask struct {
	ID          string     `json:"id"`
	WeddingID   string     `json:"wedding_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"` // pending, in-progress, completed
	Priority    string     `json:"priority"` // low, medium, high
	Assignee    string     `json:"assignee,omitempty"`
	AssigneeID  string     `json:"assignee_id,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Category    string     `json:"category,omitempty"` // catering, decoration, logistics, etc.
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CommitteeChatMessage represents a message in the committee chat room
type CommitteeChatMessage struct {
	ID        string    `json:"id"`
	WeddingID string    `json:"wedding_id"`
	SenderID  string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	SenderRole string    `json:"sender_role,omitempty"`
	Content   string    `json:"content"`
	Status    string    `json:"status"` // pending, delivered, read
	Timestamp time.Time `json:"timestamp"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	EditedAt  *time.Time `json:"edited_at,omitempty"`
}

// CommitteeAnnouncement represents an announcement visible to committee members
type CommitteeAnnouncement struct {
	ID         string    `json:"id"`
	WeddingID  string    `json:"wedding_id"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	Status     string    `json:"status"` // draft, published
	Visibility string    `json:"visibility"` // committee, public, private
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Committee represents the complete committee structure for a wedding
type Committee struct {
	WeddingID     string                   `json:"wedding_id"`
	Members       []CommitteeMember        `json:"members"`
	Tasks         []CommitteeTask          `json:"tasks"`
	ChatMessages  []CommitteeChatMessage   `json:"chat_messages"`
	Announcements []CommitteeAnnouncement  `json:"announcements"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

// InvitationRequest represents a request to create an invitation
type InvitationRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone,omitempty"`
	InvitationType string `json:"invitation_type"` // committee, guest
	Role        string `json:"role,omitempty"` // For committee members
	Category    string `json:"category,omitempty"` // For guests
}

// InvitationResponse represents the response when validating an invitation
type InvitationResponse struct {
	Valid           bool           `json:"valid"`
	WeddingID       string         `json:"wedding_id"`
	InvitationType  string         `json:"invitation_type"`
	Token           string         `json:"token"`
	Name            string         `json:"name"`
	Role            string         `json:"role,omitempty"`
	Permissions     []string       `json:"permissions,omitempty"`
	Wedding         *Wedding       `json:"wedding,omitempty"`
	Message         string         `json:"message,omitempty"`
}

// RolePermission defines what each role can do
type RolePermission struct {
	Role        string
	Permissions []string
}

// DefaultRolePermissions returns the default permissions for each role
func DefaultRolePermissions() map[string][]string {
	return map[string][]string{
		"admin": {
			"edit_wedding",
			"manage_events",
			"manage_announcements",
			"view_guests",
			"invite_committee",
			"invite_guests",
			"manage_committee",
			"view_chat",
			"delete_chat",
		},
		"committee_member": {
			"manage_events",
			"manage_announcements",
			"view_guests",
			"chat",
			"view_tasks",
		},
		"guest": {
			"view_invitation",
			"rsvp",
			"view_events",
			"view_photos",
			"view_story",
			"send_message",
		},
	}
}

// HasPermission checks if a role has a specific permission
func HasPermission(role, permission string) bool {
	perms := DefaultRolePermissions()
	rolePerms, ok := perms[role]
	if !ok {
		return false
	}
	for _, p := range rolePerms {
		if p == permission {
			return true
		}
	}
	return false
}
