package repository

import (
	"errors"
	"time"

	"weddinghub/models"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrExpired           = errors.New("invitation expired")
	ErrInvalidStatus     = errors.New("invalid status transition")
	ErrWrongRole         = errors.New("invitation role mismatch")
	ErrResetTokenInvalid = errors.New("password reset token is invalid or used")
	ErrResetTokenExpired = errors.New("password reset token has expired")
)

type Repository interface {
	CreateWedding(models.Wedding) (models.Wedding, error)
	ListWeddings() []models.Wedding
	GetWedding(id string) (models.Wedding, error)
	UpdateWedding(models.Wedding) (models.Wedding, error)
	DeleteWedding(id string) error
	// WeddingByAdminHash resolves the wedding an admin capability token administers.
	WeddingByAdminHash(hash string) (models.Wedding, error)
	// IsWeddingAdmin reports whether an account administers the wedding. Session-based
	// administration relies on this so an owner never depends on a browser-local token.
	IsWeddingAdmin(weddingID, userID string) bool
	// WeddingsForUser returns the weddings an account administers.
	WeddingsForUser(userID string) []models.Wedding
	AddInvitation(weddingID string, invitation models.Invitation) (models.Invitation, error)
	UpdateInvitation(weddingID string, invitation models.Invitation) (models.Invitation, error)
	DeleteInvitation(weddingID, invitationID string) error
	InvitationByHash(hash string) (models.Wedding, models.Invitation, error)
	InvitationByShortCodeHash(hash string) (models.Wedding, models.Invitation, error)
	PreviewByShortCodeHash(hash string) (models.Wedding, error)
	PublicWeddingPreview(weddingID string) (models.Wedding, error)
	RecordInvitationOpenByShortCodeHash(hash string, at time.Time) error
	SaveWeddingBanner(weddingID, version string, image []byte) error
	WeddingBanner(weddingID string) (version string, image []byte, err error)
	WeddingBannerVersion(weddingID string) (string, error)
	RespondToInvitation(hash string, status models.InvitationStatus, at time.Time) (models.Wedding, models.Invitation, error)
	UpdateRSVP(hash string, rsvp models.RSVP) (models.Wedding, models.RSVP, error)
	AddGuestMessage(hash string, message models.GuestMessage) (models.Wedding, models.GuestMessage, error)
	SetGuestMessageRead(weddingID, messageID string, read bool) error
	DeleteGuestMessage(weddingID, messageID string) error
	// AddCommitteeMessage appends to the private committee conversation for a wedding.
	AddCommitteeMessage(weddingID string, message models.CommitteeMessage) (models.CommitteeMessage, error)
	// CommitteeMessages returns the conversation ordered oldest first, optionally only messages created after since.
	CommitteeMessages(weddingID string, since time.Time) ([]models.CommitteeMessage, error)
	AddPlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error)
	UpdatePlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error)
	DeletePlanningTask(weddingID, taskID string) error
	ReplacePlanningTasks(weddingID string, tasks []models.PlanningTask) error
	// AddAnnouncement stores a new published or draft update for the wedding.
	AddAnnouncement(weddingID string, announcement models.Announcement) (models.Announcement, error)
	UpdateAnnouncement(weddingID string, announcement models.Announcement) (models.Announcement, error)
	DeleteAnnouncement(weddingID, announcementID string) error
	// Card customization
	UpdateCardConfig(weddingID string, config models.CardConfig) (models.CardConfig, error)
	// Dynamic committee roles and member management
	AddCommitteeRole(weddingID string, role models.CommitteeRole) (models.CommitteeRole, error)
	DeleteCommitteeRole(weddingID, roleID string) error
	UpdateCommitteeMember(weddingID string, member models.CommitteeMember) (models.CommitteeMember, error)
	DeleteCommitteeMember(weddingID, memberID string) error
	// Self-service profiles. profileKey is models.ProfileKeyAdmin or a committee member id.
	// GetProfile returns ErrNotFound when the actor has not customized a profile yet.
	GetProfile(weddingID, profileKey string) (models.Profile, error)
	UpsertProfile(profile models.Profile) (models.Profile, error)
	ListProfiles(weddingID string) ([]models.Profile, error)
	// User accounts and browser sessions. Emails are normalized before storage and lookup.
	CreateUser(user models.User) (models.User, error)
	UserByEmail(email string) (models.User, error)
	UserByID(id string) (models.User, error)
	// AddSession stores a session; SessionByHash returns ErrNotFound for unknown or expired sessions.
	AddSession(session models.Session) error
	SessionByHash(hash string) (models.Session, error)
	DeleteSession(hash string) error
	CreatePasswordResetToken(token models.PasswordResetToken) error
	CompletePasswordReset(tokenHash, passwordHash string, at time.Time) (models.User, error)
	// Platform admin audit trail. AddAuditLog appends; ListAuditLogs returns the newest first.
	// There is no update or delete: the trail is append-only by design.
	AddAuditLog(entry models.AuditLog) error
	ListAuditLogs(limit int) ([]models.AuditLog, error)
	// Platform-owner account controls and statistics.
	ListUsers(search, status string) ([]models.User, error)
	SetUserStatus(id, status string) (models.User, error)
	DeleteUser(id string) error
	SetUserPasswordHash(id, hash string) error
	EnsurePlatformAdmin(email, hash string, now time.Time) error
	PlatformStats(adminEmail string, now time.Time) (PlatformStats, error)
}

type PlatformStats struct {
	TotalUsers        int `json:"total_users"`
	ActiveUsers       int `json:"active_users"`
	SuspendedUsers    int `json:"suspended_users"`
	NewUsersThisMonth int `json:"new_users_this_month"`
	TotalWeddings     int `json:"total_weddings"`
}
