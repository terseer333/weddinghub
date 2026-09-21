package repository

import (
	"crypto/subtle"
	"sort"
	"sync"
	"time"

	"weddinghub/models"
)

// MemoryRepository is concurrency-safe. Returned aggregates are defensive copies.
type MemoryRepository struct {
	mu       sync.RWMutex
	weddings map[string]models.Wedding
	users    map[string]models.User
	emails   map[string]string
	sessions map[string]models.Session
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		weddings: make(map[string]models.Wedding),
		users:    make(map[string]models.User),
		emails:   make(map[string]string),
		sessions: make(map[string]models.Session),
	}
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
	// Invitation capabilities, guest responses, and committee state cannot be replaced through wedding content updates.
	w.Invitations = current.Invitations
	w.Guests = current.Guests
	w.CommitteeMembers = current.CommitteeMembers
	w.CommitteeRoles = current.CommitteeRoles
	if w.CardConfig == nil && current.CardConfig != nil {
		w.CardConfig = current.CardConfig
	}
	w.PlanningTasks = current.PlanningTasks
	w.CommitteeChat = current.CommitteeChat
	w.RSVPs = current.RSVPs
	w.GuestMessages = current.GuestMessages
	w.AdminTokenHash = current.AdminTokenHash
	w.CreatedAt = current.CreatedAt
	r.weddings[w.ID] = cloneWedding(w)
	return cloneWedding(w), nil
}

// WeddingByAdminHash resolves an admin capability token to its wedding using a constant-time comparison.
func (r *MemoryRepository) WeddingByAdminHash(hash string) (models.Wedding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if hash == "" {
		return models.Wedding{}, ErrNotFound
	}
	for _, w := range r.weddings {
		if len(w.AdminTokenHash) == len(hash) && subtle.ConstantTimeCompare([]byte(w.AdminTokenHash), []byte(hash)) == 1 {
			return cloneWedding(w), nil
		}
	}
	return models.Wedding{}, ErrNotFound
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
	// Acceptance materializes wedding-scoped membership for the role the invitation was issued for.
	if status == models.InvitationAccepted {
		if inv.Type.Normalized() == models.InvitationCommittee {
			if !hasCommitteeMember(w.CommitteeMembers, inv.ID) {
				memberID, err := models.NewID()
				if err != nil {
					return models.Wedding{}, models.Invitation{}, err
				}
				w.CommitteeMembers = append(w.CommitteeMembers, models.CommitteeMember{ID: memberID, InvitationID: inv.ID,
					Name: inv.GuestName, Email: inv.GuestEmail, Phone: inv.GuestPhone, Title: inv.CommitteeTitle, JoinedAt: at})
			}
		} else if !hasGuest(w.Guests, inv.ID) {
			guestID, err := models.NewID()
			if err != nil {
				return models.Wedding{}, models.Invitation{}, err
			}
			w.Guests = append(w.Guests, models.Guest{ID: guestID, InvitationID: inv.ID, Name: inv.GuestName, Email: inv.GuestEmail, Phone: inv.GuestPhone})
		}
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

// AddCommitteeMessage stores a private planning message. Authorization happens in the API layer;
// the wedding must still exist when the message is written.
func (r *MemoryRepository) AddCommitteeMessage(weddingID string, message models.CommitteeMessage) (models.CommitteeMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.CommitteeMessage{}, ErrNotFound
	}
	message.WeddingID = w.ID
	w.CommitteeChat = append(w.CommitteeChat, message)
	w.UpdatedAt = message.CreatedAt
	r.weddings[weddingID] = cloneWedding(w)
	return message, nil
}

func (r *MemoryRepository) CommitteeMessages(weddingID string, since time.Time) ([]models.CommitteeMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]models.CommitteeMessage, 0, len(w.CommitteeChat))
	for _, message := range w.CommitteeChat {
		if since.IsZero() || message.CreatedAt.After(since) {
			out = append(out, message)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *MemoryRepository) AddPlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.PlanningTask{}, ErrNotFound
	}
	for _, existing := range w.PlanningTasks {
		if existing.ID == task.ID {
			return models.PlanningTask{}, ErrConflict
		}
	}
	w.PlanningTasks = append(w.PlanningTasks, task)
	w.UpdatedAt = task.CreatedAt
	r.weddings[weddingID] = cloneWedding(w)
	return task, nil
}

// UpdatePlanningTask replaces the mutable fields of an existing task and preserves its creation metadata.
func (r *MemoryRepository) UpdatePlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.PlanningTask{}, ErrNotFound
	}
	for i := range w.PlanningTasks {
		if w.PlanningTasks[i].ID != task.ID {
			continue
		}
		task.CreatedAt = w.PlanningTasks[i].CreatedAt
		task.CreatedBy = w.PlanningTasks[i].CreatedBy
		w.PlanningTasks[i] = task
		w.UpdatedAt = task.UpdatedAt
		r.weddings[weddingID] = cloneWedding(w)
		return task, nil
	}
	return models.PlanningTask{}, ErrNotFound
}

func (r *MemoryRepository) DeletePlanningTask(weddingID, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return ErrNotFound
	}
	for i := range w.PlanningTasks {
		if w.PlanningTasks[i].ID != taskID {
			continue
		}
		w.PlanningTasks = append(w.PlanningTasks[:i:i], w.PlanningTasks[i+1:]...)
		w.UpdatedAt = time.Now().UTC()
		r.weddings[weddingID] = cloneWedding(w)
		return nil
	}
	return ErrNotFound
}

// AddAnnouncement stores an announcement (public or committee-only) in the wedding aggregate.
func (r *MemoryRepository) AddAnnouncement(weddingID string, announcement models.Announcement) (models.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.Announcement{}, ErrNotFound
	}
	for _, existing := range w.Announcements {
		if existing.ID == announcement.ID {
			return models.Announcement{}, ErrConflict
		}
	}
	w.Announcements = append(w.Announcements, announcement)
	w.UpdatedAt = time.Now().UTC()
	r.weddings[weddingID] = cloneWedding(w)
	return announcement, nil
}

func (r *MemoryRepository) UpdateAnnouncement(weddingID string, announcement models.Announcement) (models.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.Announcement{}, ErrNotFound
	}
	for i := range w.Announcements {
		if w.Announcements[i].ID != announcement.ID {
			continue
		}
		w.Announcements[i] = announcement
		w.UpdatedAt = time.Now().UTC()
		r.weddings[weddingID] = cloneWedding(w)
		return announcement, nil
	}
	return models.Announcement{}, ErrNotFound
}

func (r *MemoryRepository) DeleteAnnouncement(weddingID, announcementID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return ErrNotFound
	}
	for i := range w.Announcements {
		if w.Announcements[i].ID != announcementID {
			continue
		}
		w.Announcements = append(w.Announcements[:i:i], w.Announcements[i+1:]...)
		w.UpdatedAt = time.Now().UTC()
		r.weddings[weddingID] = cloneWedding(w)
		return nil
	}
	return ErrNotFound
}

func (r *MemoryRepository) UpdateCardConfig(weddingID string, config models.CardConfig) (models.CardConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.CardConfig{}, ErrNotFound
	}
	w.CardConfig = &config
	w.TemplateID = config.TemplateID
	w.UpdatedAt = time.Now().UTC()
	r.weddings[weddingID] = cloneWedding(w)
	return config, nil
}

func (r *MemoryRepository) AddCommitteeRole(weddingID string, role models.CommitteeRole) (models.CommitteeRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.CommitteeRole{}, ErrNotFound
	}
	for _, existing := range w.CommitteeRoles {
		if existing.ID == role.ID || (existing.Name != "" && existing.Name == role.Name) {
			return models.CommitteeRole{}, ErrConflict
		}
	}
	w.CommitteeRoles = append(w.CommitteeRoles, role)
	w.UpdatedAt = time.Now().UTC()
	r.weddings[weddingID] = cloneWedding(w)
	return role, nil
}

func (r *MemoryRepository) DeleteCommitteeRole(weddingID, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return ErrNotFound
	}
	for i := range w.CommitteeRoles {
		if w.CommitteeRoles[i].ID != roleID {
			continue
		}
		w.CommitteeRoles = append(w.CommitteeRoles[:i:i], w.CommitteeRoles[i+1:]...)
		w.UpdatedAt = time.Now().UTC()
		r.weddings[weddingID] = cloneWedding(w)
		return nil
	}
	return ErrNotFound
}

func (r *MemoryRepository) UpdateCommitteeMember(weddingID string, member models.CommitteeMember) (models.CommitteeMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return models.CommitteeMember{}, ErrNotFound
	}
	for i := range w.CommitteeMembers {
		if w.CommitteeMembers[i].ID != member.ID {
			continue
		}
		member.InvitationID = w.CommitteeMembers[i].InvitationID
		member.JoinedAt = w.CommitteeMembers[i].JoinedAt
		w.CommitteeMembers[i] = member
		w.UpdatedAt = time.Now().UTC()
		r.weddings[weddingID] = cloneWedding(w)
		return member, nil
	}
	return models.CommitteeMember{}, ErrNotFound
}

func (r *MemoryRepository) DeleteCommitteeMember(weddingID, memberID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.weddings[weddingID]
	if !ok {
		return ErrNotFound
	}
	for i := range w.CommitteeMembers {
		if w.CommitteeMembers[i].ID != memberID {
			continue
		}
		w.CommitteeMembers = append(w.CommitteeMembers[:i:i], w.CommitteeMembers[i+1:]...)
		w.UpdatedAt = time.Now().UTC()
		r.weddings[weddingID] = cloneWedding(w)
		return nil
	}
	return ErrNotFound
}

func (r *MemoryRepository) CreateUser(user models.User) (models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email := models.NormalizeEmail(user.Email)
	if user.ID == "" || email == "" {
		return models.User{}, ErrConflict
	}
	if _, exists := r.users[user.ID]; exists {
		return models.User{}, ErrConflict
	}
	if _, exists := r.emails[email]; exists {
		return models.User{}, ErrConflict
	}
	user.Email = email
	r.users[user.ID] = user
	r.emails[email] = user.ID
	return user, nil
}

func (r *MemoryRepository) UserByEmail(email string) (models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.emails[models.NormalizeEmail(email)]
	if !ok {
		return models.User{}, ErrNotFound
	}
	user, ok := r.users[id]
	if !ok {
		return models.User{}, ErrNotFound
	}
	return user, nil
}

func (r *MemoryRepository) UserByID(id string) (models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return models.User{}, ErrNotFound
	}
	return user, nil
}

func (r *MemoryRepository) AddSession(session models.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session.TokenHash == "" || session.UserID == "" {
		return ErrConflict
	}
	r.sessions[session.TokenHash] = session
	return nil
}

// SessionByHash treats an expired session as missing and drops it.
func (r *MemoryRepository) SessionByHash(hash string) (models.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[hash]
	if !ok {
		return models.Session{}, ErrNotFound
	}
	if !session.ExpiresAt.IsZero() && time.Now().UTC().After(session.ExpiresAt) {
		delete(r.sessions, hash)
		return models.Session{}, ErrNotFound
	}
	return session, nil
}

func (r *MemoryRepository) DeleteSession(hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[hash]; !ok {
		return ErrNotFound
	}
	delete(r.sessions, hash)
	return nil
}

func (r *MemoryRepository) findInvitation(hash string) (models.Wedding, models.Invitation, bool) {
	if hash == "" {
		return models.Wedding{}, models.Invitation{}, false
	}
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

func hasCommitteeMember(members []models.CommitteeMember, invitationID string) bool {
	for _, member := range members {
		if member.InvitationID == invitationID {
			return true
		}
	}
	return false
}

func cloneWedding(w models.Wedding) models.Wedding {
	w.Admins = append([]models.Admin(nil), w.Admins...)
	w.Guests = append([]models.Guest(nil), w.Guests...)
	w.CommitteeMembers = append([]models.CommitteeMember(nil), w.CommitteeMembers...)
	w.CommitteeRoles = append([]models.CommitteeRole(nil), w.CommitteeRoles...)
	if w.CardConfig != nil {
		cfg := *w.CardConfig
		if cfg.CustomStyles != nil {
			m := make(map[string]string, len(cfg.CustomStyles))
			for k, v := range cfg.CustomStyles {
				m[k] = v
			}
			cfg.CustomStyles = m
		}
		w.CardConfig = &cfg
	}
	w.Invitations = append([]models.Invitation(nil), w.Invitations...)
	w.Events = append([]models.Event(nil), w.Events...)
	w.Photos = append([]models.Photo(nil), w.Photos...)
	w.StorySections = append([]models.StorySection(nil), w.StorySections...)
	w.Announcements = append([]models.Announcement(nil), w.Announcements...)
	w.PlanningTasks = append([]models.PlanningTask(nil), w.PlanningTasks...)
	w.CommitteeChat = append([]models.CommitteeMessage(nil), w.CommitteeChat...)
	w.RSVPs = append([]models.RSVP(nil), w.RSVPs...)
	w.GuestMessages = append([]models.GuestMessage(nil), w.GuestMessages...)
	return w
}
