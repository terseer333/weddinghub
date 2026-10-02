package api

import (
	"net/http"
	"sort"
	"time"

	"weddinghub/models"
)

type platformWedding struct {
	ID         string                   `json:"id"`
	Slug       string                   `json:"slug"`
	Title      string                   `json:"title"`
	PartnerOne string                   `json:"partner_one"`
	PartnerTwo string                   `json:"partner_two"`
	Date       *time.Time               `json:"date,omitempty"`
	Status     models.PublicationStatus `json:"status"`
	CreatedAt  time.Time                `json:"created_at"`
	Owner      *models.User             `json:"owner,omitempty"`
}

func (a *API) adminDashboard(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	stats, err := a.repo.PlatformStats(a.adminEmail, a.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load platform statistics")
		return
	}
	users, err := a.repo.ListUsers("", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load accounts")
		return
	}
	recentUsers := make([]models.User, 0, 5)
	for _, user := range users {
		if models.NormalizeEmail(user.Email) == a.adminEmail {
			continue
		}
		user.PasswordHash = ""
		recentUsers = append(recentUsers, user)
		if len(recentUsers) == 5 {
			break
		}
	}
	weddings := a.repo.ListWeddings()
	sort.Slice(weddings, func(i, j int) bool { return weddings[i].CreatedAt.After(weddings[j].CreatedAt) })
	recentWeddings := make([]platformWedding, 0, 5)
	for _, wedding := range weddings {
		recentWeddings = append(recentWeddings, a.platformWedding(wedding))
		if len(recentWeddings) == 5 {
			break
		}
	}
	admin.PasswordHash = ""
	writeJSON(w, http.StatusOK, map[string]any{"administrator": admin, "stats": stats, "recent_users": recentUsers, "recent_weddings": recentWeddings})
}

func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdminUser(w, r); !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "active" && status != "suspended" {
		writeError(w, http.StatusBadRequest, "invalid account status filter")
		return
	}
	users, err := a.repo.ListUsers(r.URL.Query().Get("search"), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load accounts")
		return
	}
	out := make([]models.User, 0, len(users))
	for _, user := range users {
		if models.NormalizeEmail(user.Email) == a.adminEmail {
			continue
		}
		user.PasswordHash = ""
		out = append(out, user)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (a *API) adminSetUserStatus(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil || (input.Status != "active" && input.Status != "suspended") {
		writeError(w, http.StatusBadRequest, "status must be active or suspended")
		return
	}
	id := r.PathValue("userID")
	user, err := a.repo.UserByID(id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if models.NormalizeEmail(user.Email) == a.adminEmail {
		writeError(w, http.StatusBadRequest, "the configured administrator cannot be suspended")
		return
	}
	user, err = a.repo.SetUserStatus(id, input.Status)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	a.recordAudit(r, models.AuditLog{ActorID: admin.ID, ActorEmail: admin.Email, Action: "user.status_changed", ResourceType: "user", ResourceID: id, Metadata: map[string]string{"status": input.Status}})
	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *API) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("userID")
	user, err := a.repo.UserByID(id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if id == admin.ID || models.NormalizeEmail(user.Email) == a.adminEmail {
		writeError(w, http.StatusBadRequest, "the configured administrator cannot be deleted")
		return
	}
	if err := a.repo.DeleteUser(id); err != nil {
		writeRepositoryError(w, err)
		return
	}
	a.recordAudit(r, models.AuditLog{ActorID: admin.ID, ActorEmail: admin.Email, Action: "user.deleted", ResourceType: "user", ResourceID: id, Metadata: map[string]string{"email": user.Email}})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) adminWeddings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdminUser(w, r); !ok {
		return
	}
	weddings := a.repo.ListWeddings()
	sort.Slice(weddings, func(i, j int) bool { return weddings[i].CreatedAt.After(weddings[j].CreatedAt) })
	out := make([]platformWedding, 0, len(weddings))
	for _, wedding := range weddings {
		out = append(out, a.platformWedding(wedding))
	}
	writeJSON(w, http.StatusOK, map[string]any{"weddings": out})
}

func (a *API) platformWedding(wedding models.Wedding) platformWedding {
	result := platformWedding{ID: wedding.ID, Slug: wedding.Slug, Title: wedding.Title, PartnerOne: wedding.PartnerOne, PartnerTwo: wedding.PartnerTwo, Date: wedding.Date, Status: wedding.Status, CreatedAt: wedding.CreatedAt}
	for _, owner := range wedding.Admins {
		if owner.Role != models.RoleOwner {
			continue
		}
		user, err := a.repo.UserByID(owner.UserID)
		if err == nil {
			user.PasswordHash = ""
			result.Owner = &user
		}
		break
	}
	return result
}

func (a *API) adminSetWeddingStatus(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Status models.PublicationStatus `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil || !input.Status.Valid() {
		writeError(w, http.StatusBadRequest, "status must be draft, published, or hidden")
		return
	}
	wedding, err := a.repo.GetWedding(r.PathValue("weddingID"))
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	wedding.Status = input.Status
	wedding.UpdatedAt = a.now()
	updated, err := a.repo.UpdateWedding(wedding)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	a.recordAudit(r, models.AuditLog{ActorID: admin.ID, ActorEmail: admin.Email, Action: "wedding.status_changed", ResourceType: "wedding", ResourceID: wedding.ID, Metadata: map[string]string{"status": string(input.Status)}})
	writeJSON(w, http.StatusOK, a.platformWedding(updated))
}

func (a *API) adminDeleteWedding(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("weddingID")
	wedding, err := a.repo.GetWedding(id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if err := a.repo.DeleteWedding(id); err != nil {
		writeRepositoryError(w, err)
		return
	}
	a.recordAudit(r, models.AuditLog{ActorID: admin.ID, ActorEmail: admin.Email, Action: "wedding.deleted", ResourceType: "wedding", ResourceID: id, Metadata: map[string]string{"slug": wedding.Slug}})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) adminAuditLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdminUser(w, r); !ok {
		return
	}
	logs, err := a.repo.ListAuditLogs(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load audit log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (a *API) adminLoginAttempts(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attempts": a.loginGuard.recentAttempts(100)})
}
