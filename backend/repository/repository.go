package repository

import (
	"errors"
	"time"

	"weddinghub/models"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrExpired       = errors.New("invitation expired")
	ErrInvalidStatus = errors.New("invalid status transition")
	ErrWrongRole     = errors.New("invitation role mismatch")
)

type Repository interface {
	CreateWedding(models.Wedding) (models.Wedding, error)
	ListWeddings() []models.Wedding
	GetWedding(id string) (models.Wedding, error)
	UpdateWedding(models.Wedding) (models.Wedding, error)
	DeleteWedding(id string) error
	// WeddingByAdminHash resolves the wedding an admin capability token administers.
	WeddingByAdminHash(hash string) (models.Wedding, error)
	AddInvitation(weddingID string, invitation models.Invitation) (models.Invitation, error)
	InvitationByHash(hash string) (models.Wedding, models.Invitation, error)
	RespondToInvitation(hash string, status models.InvitationStatus, at time.Time) (models.Wedding, models.Invitation, error)
	UpdateRSVP(hash string, rsvp models.RSVP) (models.Wedding, models.RSVP, error)
	AddGuestMessage(hash string, message models.GuestMessage) (models.Wedding, models.GuestMessage, error)
	// AddCommitteeMessage appends to the private committee conversation for a wedding.
	AddCommitteeMessage(weddingID string, message models.CommitteeMessage) (models.CommitteeMessage, error)
	// CommitteeMessages returns the conversation ordered oldest first, optionally only messages created after since.
	CommitteeMessages(weddingID string, since time.Time) ([]models.CommitteeMessage, error)
	AddPlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error)
	UpdatePlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error)
	DeletePlanningTask(weddingID, taskID string) error
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
}
