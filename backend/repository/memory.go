package repository

import (
	"crypto/subtle"
	"sync"
	"time"

	"weddinghub/models"
)

// MemoryRepository is concurrency-safe. Returned aggregates are defensive copies.
type MemoryRepository struct {
	mu       sync.RWMutex
	weddings map[string]models.Wedding
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{weddings: make(map[string]models.Wedding)}
}

func (r *MemoryRepository) CreateWedding(w models.Wedding) (models.Wedding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.weddings[w.ID]; exists || r.slugExists(w.Slug, "") {
		return models.Wedding{}, ErrConflict
	}
	r.weddings[w.ID] = cloneWedding(w)
	return cloneWedding(w), nil
}

func (r *MemoryRepository) ListWeddings() []models.Wedding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]models.Wedding, 0, len(r.weddings))
	for _, w := range r.weddings {
		out = append(out, cloneWedding(w))
	}
	return out
}

func (r *MemoryRepository) GetWedding(id string) (models.Wedding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.weddings[id]
	if !ok {
		return models.Wedding{}, ErrNotFound
	}
	return cloneWedding(w), nil
}

func (r *MemoryRepository) UpdateWedding(w models.Wedding) (models.Wedding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.weddings[w.ID]
	if !ok {
		return models.Wedding{}, ErrNotFound
	}
	if r.slugExists(w.Slug, w.ID) {
		return models.Wedding{}, ErrConflict
	}
	// Invitation capabilities and guest responses cannot be replaced through wedding content updates.
	w.Invitations = current.Invitations
	w.Guests = current.Guests
	w.RSVPs = current.RSVPs
	w.GuestMessages = current.GuestMessages
	w.CreatedAt = current.CreatedAt
	r.weddings[w.ID] = cloneWedding(w)
	return cloneWedding(w), nil
}

func (r *MemoryRepository) DeleteWedding(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.weddings[id]; !ok {
		return ErrNotFound
	}
	delete(r.weddings, id)
	return nil
}

func (r *MemoryRepository) AddInvitation(weddingID string, inv models.Invitation) (models.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.Invitation{}, ErrNotFound
	}
	for _, existing := range w.Invitations {
		if existing.TokenHash == inv.TokenHash || existing.ID == inv.ID {
			return models.Invitation{}, ErrConflict
		}
	}
	w.Invitations = append(w.Invitations, inv)
	w.UpdatedAt = time.Now().UTC()
	r.weddings[weddingID] = cloneWedding(w)
	return inv, nil
}

func (r *MemoryRepository) InvitationByHash(hash string) (models.Wedding, models.Invitation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, inv, ok := r.findInvitation(hash)
	if !ok {
		return models.Wedding{}, models.Invitation{}, ErrNotFound
	}
	if inv.ExpiresAt != nil && time.Now().UTC().After(*inv.ExpiresAt) {
		return models.Wedding{}, models.Invitation{}, ErrExpired
	}
	return cloneWedding(w), inv, nil
}

func (r *MemoryRepository) RespondToInvitation(hash string, status models.InvitationStatus, at time.Time) (models.Wedding, models.Invitation, error) {
	if status != models.InvitationAccepted && status != models.InvitationDeclined {
		return models.Wedding{}, models.Invitation{}, ErrInvalidStatus
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	w, inv, ok := r.findInvitation(hash)
	if !ok {
		return models.Wedding{}, models.Invitation{}, ErrNotFound
	}
	if inv.ExpiresAt != nil && at.After(*inv.ExpiresAt) {
		return models.Wedding{}, models.Invitation{}, ErrExpired
	}
	if inv.Status != models.InvitationPending && inv.Status != status {
		return models.Wedding{}, models.Invitation{}, ErrInvalidStatus
	}
	for i := range w.Invitations {
		if w.Invitations[i].ID != inv.ID {
			continue
		}
		w.Invitations[i].Status = status
		w.Invitations[i].RespondedAt = &at
		inv = w.Invitations[i]
		break
	}
	if status == models.InvitationAccepted && !hasGuest(w.Guests, inv.ID) {
		guestID, err := models.NewID()
		if err != nil {
			return models.Wedding{}, models.Invitation{}, err
		}
		w.Guests = append(w.Guests, models.Guest{ID: guestID, InvitationID: inv.ID, Name: inv.GuestName, Email: inv.GuestEmail})
	}
	w.UpdatedAt = at
	r.weddings[w.ID] = cloneWedding(w)
	return cloneWedding(w), inv, nil
}

func (r *MemoryRepository) UpdateRSVP(hash string, response models.RSVP) (models.Wedding, models.RSVP, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, inv, ok := r.findInvitation(hash)
	if !ok {
		return models.Wedding{}, models.RSVP{}, ErrNotFound
	}
	if inv.ExpiresAt != nil && response.UpdatedAt.After(*inv.ExpiresAt) {
		return models.Wedding{}, models.RSVP{}, ErrExpired
	}
	if inv.Status != models.InvitationAccepted {
		return models.Wedding{}, models.RSVP{}, ErrInvalidStatus
	}
	response.InvitationID = inv.ID
	updated := false
	for i := range w.RSVPs {
		if w.RSVPs[i].InvitationID == inv.ID {
			w.RSVPs[i] = response
			updated = true
			break
		}
	}
	if !updated {
		w.RSVPs = append(w.RSVPs, response)
	}
	w.UpdatedAt = response.UpdatedAt
	r.weddings[w.ID] = cloneWedding(w)
	return cloneWedding(w), response, nil
}

func (r *MemoryRepository) AddGuestMessage(hash string, message models.GuestMessage) (models.Wedding, models.GuestMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, inv, ok := r.findInvitation(hash)
	if !ok {
		return models.Wedding{}, models.GuestMessage{}, ErrNotFound
	}
	if inv.ExpiresAt != nil && message.CreatedAt.After(*inv.ExpiresAt) {
		return models.Wedding{}, models.GuestMessage{}, ErrExpired
	}
	if inv.Status != models.InvitationAccepted {
		return models.Wedding{}, models.GuestMessage{}, ErrInvalidStatus
	}
	message.WeddingID = w.ID
	message.InvitationID = inv.ID
	w.GuestMessages = append(w.GuestMessages, message)
	w.UpdatedAt = message.CreatedAt
	r.weddings[w.ID] = cloneWedding(w)
	return cloneWedding(w), message, nil
}

func (r *MemoryRepository) findInvitation(hash string) (models.Wedding, models.Invitation, bool) {
	for _, w := range r.weddings {
		for _, inv := range w.Invitations {
			if len(inv.TokenHash) == len(hash) && subtle.ConstantTimeCompare([]byte(inv.TokenHash), []byte(hash)) == 1 {
				return w, inv, true
			}
		}
	}
	return models.Wedding{}, models.Invitation{}, false
}

func (r *MemoryRepository) slugExists(slug, exceptID string) bool {
	for id, w := range r.weddings {
		if id != exceptID && w.Slug == slug {
			return true
		}
	}
	return false
}

func hasGuest(guests []models.Guest, invitationID string) bool {
	for _, guest := range guests {
		if guest.InvitationID == invitationID {
			return true
		}
	}
	return false
}

func cloneWedding(w models.Wedding) models.Wedding {
	w.Admins = append([]models.Admin(nil), w.Admins...)
	w.Guests = append([]models.Guest(nil), w.Guests...)
	w.Invitations = append([]models.Invitation(nil), w.Invitations...)
	w.Events = append([]models.Event(nil), w.Events...)
	w.Photos = append([]models.Photo(nil), w.Photos...)
	w.StorySections = append([]models.StorySection(nil), w.StorySections...)
	w.Announcements = append([]models.Announcement(nil), w.Announcements...)
	w.RSVPs = append([]models.RSVP(nil), w.RSVPs...)
	w.GuestMessages = append([]models.GuestMessage(nil), w.GuestMessages...)
	return w
}
