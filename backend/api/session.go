package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"weddinghub/models"
	"weddinghub/repository"
)

// sessionTTL is how long a browser session stays valid.
const sessionTTL = 30 * 24 * time.Hour

// dummyPasswordHash equalizes login timing for unknown accounts so the endpoint does
// not reveal which email addresses have an account.
var dummyPasswordHash = func() string {
	hash, err := models.HashPassword("weddinghub-invalid-password")
	if err != nil {
		return ""
	}
	return hash
}()

type credentialsRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// sessionResponse returns the account and its one-time session token. models.User
// never serializes the password hash.
type sessionResponse struct {
	User         models.User `json:"user"`
	SessionToken string      `json:"session_token"`
	ExpiresAt    time.Time   `json:"expires_at"`
}

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	var input credentialsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := models.NormalizeEmail(input.Email)
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "a valid email address is required")
		return
	}
	if len(input.Password) < models.MinPasswordLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("password must be at least %d characters", models.MinPasswordLength))
		return
	}
	hash, err := models.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not secure this password")
		return
	}
	userID := mustID(w)
	if userID == "" {
		return
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = displayNameFromEmail(email)
	}
	user, err := a.repo.CreateUser(models.User{
		ID: userID, Email: email, DisplayName: displayName,
		Role: models.RoleOwner, PasswordHash: hash, CreatedAt: a.now(),
	})
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "an account with this email already exists")
			return
		}
		writeRepositoryError(w, err)
		return
	}
	a.issueSession(w, user, http.StatusCreated)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var input credentialsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := models.NormalizeEmail(input.Email)
	// An unknown email gets the same status and comparable work as a wrong password.
	user, err := a.repo.UserByEmail(email)
	if err != nil {
		_ = models.VerifyPassword(dummyPasswordHash, input.Password)
		writeError(w, http.StatusUnauthorized, "email or password is incorrect")
		return
	}
	if err := models.VerifyPassword(user.PasswordHash, input.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "email or password is incorrect")
		return
	}
	a.issueSession(w, user, http.StatusOK)
}

func (a *API) currentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" || !validToken(token) {
		writeUnauthorized(w)
		return
	}
	if err := a.repo.DeleteSession(models.HashToken(token)); err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// issueSession mints a session token and reveals it once, alongside the account.
func (a *API) issueSession(w http.ResponseWriter, user models.User, status int) {
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create a session")
		return
	}
	now := a.now()
	expiresAt := now.Add(sessionTTL)
	if err := a.repo.AddSession(models.Session{TokenHash: hash, UserID: user.ID, CreatedAt: now, ExpiresAt: expiresAt}); err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, status, sessionResponse{User: user, SessionToken: token, ExpiresAt: expiresAt})
}

// requireUser resolves a browser session token to its account. Session tokens are
// separate from invitation and wedding-admin capability tokens.
func (a *API) requireUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	token := bearerToken(r)
	if token == "" || !validToken(token) {
		writeUnauthorized(w)
		return models.User{}, false
	}
	session, err := a.repo.SessionByHash(models.HashToken(token))
	if err != nil {
		writeUnauthorized(w)
		return models.User{}, false
	}
	user, err := a.repo.UserByID(session.UserID)
	if err != nil {
		writeUnauthorized(w)
		return models.User{}, false
	}
	return user, true
}

// validEmail applies a deliberately small shape check: one @, a dotted domain, no spaces.
func validEmail(email string) bool {
	if email == "" || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

// displayNameFromEmail derives a readable name such as "Ada Lovelace" from
// "ada.lovelace@example.com".
func displayNameFromEmail(email string) string {
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	local = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(local)
	words := strings.Fields(local)
	if len(words) == 0 {
		return "Wedding Admin"
	}
	for i, word := range words {
		letters := []rune(word)
		words[i] = strings.ToUpper(string(letters[0])) + string(letters[1:])
	}
	return strings.Join(words, " ")
}
