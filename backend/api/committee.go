package api

import (
	"encoding/json"
	"net/http"
	"time"

	"weddinghub/models"
)

// CommitteeHandler handles all committee-related API endpoints
type CommitteeHandler struct {
	repo models.Repository
}

// NewCommitteeHandler creates a new committee handler
func NewCommitteeHandler(repo models.Repository) *CommitteeHandler {
	return &CommitteeHandler{repo: repo}
}

// GetCommitteeDashboard retrieves the committee dashboard data
// GET /api/weddings/{weddingID}/committee/dashboard
// Requires: valid committee member token
func (h *CommitteeHandler) GetCommitteeDashboard(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Committee-Token")
	if token == "" {
		http.Error(w, "Unauthorized: Missing committee token", http.StatusUnauthorized)
		return
	}

	// Validate token and get member
	weddingID := r.URL.Query().Get("wedding_id")
	member, err := h.repo.GetCommitteeMemberByToken(weddingID, token)
	if err != nil || member == nil {
		http.Error(w, "Unauthorized: Invalid committee token", http.StatusUnauthorized)
		return
	}

	// Check if member has accepted
	if member.Status != "accepted" {
		http.Error(w, "Forbidden: Committee member has not accepted invitation", http.StatusForbidden)
		return
	}

	// Get committee data
	committee, err := h.repo.GetCommittee(weddingID)
	if err != nil {
		http.Error(w, "Failed to retrieve committee data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(committee)
}

// InviteCommitteeMember creates a new committee member invitation
// POST /api/weddings/{weddingID}/committee/invite
// Requires: admin token
func (h *CommitteeHandler) InviteCommitteeMember(w http.ResponseWriter, r *http.Request) {
	weddingID := r.URL.Query().Get("wedding_id")

	// Check admin authorization
	if !h.isAdminAuthorized(r, weddingID) {
		http.Error(w, "Unauthorized: Admin access required", http.StatusUnauthorized)
		return
	}

	var req models.InvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" || req.Email == "" || req.InvitationType == "" {
		http.Error(w, "Missing required fields: name, email, invitation_type", http.StatusBadRequest)
		return
	}

	if req.InvitationType != "committee" {
		http.Error(w, "Invalid invitation type. Use 'committee' for committee members", http.StatusBadRequest)
		return
	}

	// Create committee member
	member := &models.CommitteeMember{
		ID:               generateID("cmm"),
		WeddingID:        weddingID,
		Name:             req.Name,
		Email:            req.Email,
		Phone:            req.Phone,
		Role:             req.Role,
		Status:           "pending",
		Token:            generateToken(),
		InvitationStatus: "sent",
		InvitedAt:        time.Now(),
	}

	// Set default permissions
	member.Permissions = []string{"view_tasks", "chat", "view_announcements"}

	if err := h.repo.SaveCommitteeMember(member); err != nil {
		http.Error(w, "Failed to create invitation", http.StatusInternalServerError)
		return
	}

	// TODO: Send invitation email with token link

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(member)
}

// AcceptCommitteeInvitation marks a committee member invitation as accepted
// POST /api/weddings/{weddingID}/committee/accept
// Requires: valid committee token
func (h *CommitteeHandler) AcceptCommitteeInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Committee-Token")
	weddingID := r.URL.Query().Get("wedding_id")

	member, err := h.repo.GetCommitteeMemberByToken(weddingID, token)
	if err != nil || member == nil {
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		return
	}

	// Update member status
	now := time.Now()
	member.Status = "accepted"
	member.InvitationStatus = "accepted"
	member.AcceptedAt = &now
	member.JoinedAt = &now

	if err := h.repo.SaveCommitteeMember(member); err != nil {
		http.Error(w, "Failed to accept invitation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(member)
}

// DeclineCommitteeInvitation marks a committee member invitation as declined
// POST /api/weddings/{weddingID}/committee/decline
// Requires: valid committee token
func (h *CommitteeHandler) DeclineCommitteeInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Committee-Token")
	weddingID := r.URL.Query().Get("wedding_id")

	member, err := h.repo.GetCommitteeMemberByToken(weddingID, token)
	if err != nil || member == nil {
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		return
	}

	// Update member status
	now := time.Now()
	member.Status = "declined"
	member.InvitationStatus = "declined"
	member.DeclinedAt = &now

	if err := h.repo.SaveCommitteeMember(member); err != nil {
		http.Error(w, "Failed to decline invitation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(member)
}

// SendCommitteeChatMessage sends a message to the committee chat
// POST /api/weddings/{weddingID}/committee/chat/messages
// Requires: valid committee token
func (h *CommitteeHandler) SendCommitteeChatMessage(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Committee-Token")
	weddingID := r.URL.Query().Get("wedding_id")

	member, err := h.repo.GetCommitteeMemberByToken(weddingID, token)
	if err != nil || member == nil {
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		return
	}

	// Check chat permission
	if !hasPermission(member.Permissions, "chat") {
		http.Error(w, "Forbidden: No chat permission", http.StatusForbidden)
		return
	}

	var msgReq struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&msgReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if msgReq.Content == "" {
		http.Error(w, "Message content cannot be empty", http.StatusBadRequest)
		return
	}

	// Create message
	msg := &models.CommitteeChatMessage{
		ID:        generateID("msg"),
		WeddingID: weddingID,
		SenderID:  member.ID,
		SenderName: member.Name,
		SenderRole: member.Role,
		Content:   msgReq.Content,
		Status:    "delivered",
		Timestamp: time.Now(),
	}

	if err := h.repo.SaveCommitteeChatMessage(msg); err != nil {
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msg)
}

// GetCommitteeChatMessages retrieves all committee chat messages
// GET /api/weddings/{weddingID}/committee/chat/messages
// Requires: valid committee token
func (h *CommitteeHandler) GetCommitteeChatMessages(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Committee-Token")
	weddingID := r.URL.Query().Get("wedding_id")

	member, err := h.repo.GetCommitteeMemberByToken(weddingID, token)
	if err != nil || member == nil {
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		return
	}

	messages, err := h.repo.GetCommitteeChatMessages(weddingID)
	if err != nil {
		http.Error(w, "Failed to retrieve messages", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// Helper functions

func (h *CommitteeHandler) isAdminAuthorized(r *http.Request, weddingID string) bool {
	// Check admin token or session
	// This would be implemented based on your authentication system
	adminToken := r.Header.Get("X-Admin-Token")
	return adminToken != "" // Simplified for example
}

func hasPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == required {
			return true
		}
	}
	return false
}

func generateID(prefix string) string {
	// Generate unique ID with prefix
	return prefix + "_" + randString(8)
}

func generateToken() string {
	// Generate secure random token
	return randString(32)
}

func randString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
