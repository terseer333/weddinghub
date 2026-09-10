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
)

type Repository interface {
	CreateWedding(models.Wedding) (models.Wedding, error)
	ListWeddings() []models.Wedding
	GetWedding(id string) (models.Wedding, error)
	UpdateWedding(models.Wedding) (models.Wedding, error)
	DeleteWedding(id string) error
	AddInvitation(weddingID string, invitation models.Invitation) (models.Invitation, error)
	InvitationByHash(hash string) (models.Wedding, models.Invitation, error)
	RespondToInvitation(hash string, status models.InvitationStatus, at time.Time) (models.Wedding, models.Invitation, error)
	UpdateRSVP(hash string, rsvp models.RSVP) (models.Wedding, models.RSVP, error)
	AddGuestMessage(hash string, message models.GuestMessage) (models.Wedding, models.GuestMessage, error)
}
