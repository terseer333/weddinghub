package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"weddinghub/models"
	"weddinghub/repository"
)

// actor is the authenticated caller resolved for a single wedding. Roles are wedding-scoped:
// the same person can be a committee member for one wedding and a guest for another, so an
// actor is only ever produced together with the wedding it applies to.
type actor struct {
	Role         models.Role
	WeddingID    string
	InvitationID string
	MemberID     string
	Name         string
}

func (a actor) isAdmin() bool { return a.Role == models.RoleAdmin }

// canReadCommittee reports whether the actor may see private planning content.
// Guests never satisfy this, which is what keeps committee data out of guest responses.
func (a actor) canReadCommittee() bool {
	return a.Role == models.RoleAdmin || a.Role == models.RoleCommitteeMember
}

// bearerToken extracts a capability token from the Authorization header.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if len(header) < 7 || !strings.EqualFold(header[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func constantTimeMatch(a, b string) bool {
	return a != "" && len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// requireAdmin authenticates a wedding administrator for weddingID.
// Administration always requires the wedding's admin capability token; a wedding without an
// admin token hash cannot be administered at all rather than falling open.
func (a *API) requireAdmin(w http.ResponseWriter, r *http.Request, weddingID string) (models.Wedding, bool) {
	token := bearerToken(r)
	if token == "" || !validToken(token) {
		writeUnauthorized(w)
		return models.Wedding{}, false
	}
	wedding, err := a.repo.GetWedding(weddingID)
	if err != nil {
		writeRepositoryError(w, err)
		return models.Wedding{}, false
	}
	if !constantTimeMatch(wedding.AdminTokenHash, models.HashToken(token)) {
		writeError(w, http.StatusForbidden, "wedding administration requires this wedding's admin token")
		return models.Wedding{}, false
	}
	return wedding, true
}

// requireCommittee authenticates an admin or an accepted committee member of weddingID.
// Guest invitation tokens are rejected here, so committee routes are protected by the backend
// regardless of what the frontend chooses to display.
func (a *API) requireCommittee(w http.ResponseWriter, r *http.Request, weddingID string) (models.Wedding, actor, bool) {
	token := bearerToken(r)
	if token == "" || !validToken(token) {
		writeUnauthorized(w)
		return models.Wedding{}, actor{}, false
	}
	wedding, err := a.repo.GetWedding(weddingID)
	if err != nil {
		writeRepositoryError(w, err)
		return models.Wedding{}, actor{}, false
	}
	hash := models.HashToken(token)

	if constantTimeMatch(wedding.AdminTokenHash, hash) {
		name := strings.TrimSpace(wedding.PartnerOne + " & " + wedding.PartnerTwo)
		if name == "&" || name == "" {
			name = "Wedding admin"
		}
		return wedding, actor{Role: models.RoleAdmin, WeddingID: wedding.ID, Name: name}, true
	}

	inviteWedding, invitation, err := a.repo.InvitationByHash(hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// An unknown token is not told apart from a token for another wedding.
			writeError(w, http.StatusForbidden, "committee access requires an accepted committee invitation for this wedding")
			return models.Wedding{}, actor{}, false
		}
		writeRepositoryError(w, err)
		return models.Wedding{}, actor{}, false
	}
	if inviteWedding.ID != wedding.ID || invitation.Type.Normalized() != models.InvitationCommittee {
		writeError(w, http.StatusForbidden, "committee access requires an accepted committee invitation for this wedding")
		return models.Wedding{}, actor{}, false
	}
	if invitation.Status != models.InvitationAccepted {
		writeError(w, http.StatusForbidden, "accept your committee invitation to use the planning workspace")
		return models.Wedding{}, actor{}, false
	}

	resolved := actor{Role: models.RoleCommitteeMember, WeddingID: wedding.ID, InvitationID: invitation.ID, Name: invitation.GuestName}
	for _, member := range wedding.CommitteeMembers {
		if member.InvitationID == invitation.ID {
			resolved.MemberID = member.ID
			if strings.TrimSpace(member.Name) != "" {
				resolved.Name = member.Name
			}
			break
		}
	}
	return wedding, resolved, true
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "a valid capability token is required")
}
