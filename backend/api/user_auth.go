package api

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"weddinghub/models"
	"weddinghub/repository"
)

const (
	loginCodeTTL         = 10 * time.Minute
	loginCodeCooldown    = time.Minute
	loginCodeMaxAttempts = 5
	sessionTTL           = 30 * 24 * time.Hour
	maxEmailLength       = 254
	maxFullNameLength    = 200
	// Password guessing burns a code request, so the lockout budget applies to
	// the password step: five misses lock the account for ten minutes.
	passwordMaxAttempts = 5
	passwordLockout     = 10 * time.Minute
)

type registerRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

func (a *API) registerUser(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Email = models.NormalizeEmail(input.Email)
	input.FullName = strings.TrimSpace(input.FullName)
	if len(input.Email) > maxEmailLength || !models.ValidEmail(input.Email) {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if input.FullName == "" || len(input.FullName) > maxFullNameLength {
		writeError(w, http.StatusBadRequest, "full_name is required and must be at most 200 characters")
		return
	}
	if !models.ValidPassword(input.Password) {
		writeError(w, http.StatusBadRequest, "password must be 10-128 characters and include a letter and a digit")
		return
	}
	passwordHash, err := models.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not secure this account")
		return
	}
	id := mustID(w)
	if id == "" {
		return
	}
	now := a.now()
	user := models.User{ID: id, Email: input.Email, FullName: input.FullName, PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now}
	created, err := a.repo.CreateUser(user)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

type loginStartRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginStartResponse struct {
	ExpiresIn   int `json:"expires_in"`
	ResendAfter int `json:"resend_after"`
}

// startLogin checks the account password and then emails a fresh verification
// code: the code is a second factor, not a password replacement. Every failure
// at this step reports the same generic error, and unknown emails still run a
// dummy argon2id verification, so neither the response nor its timing reveals
// whether an email belongs to an account.
func (a *API) startLogin(w http.ResponseWriter, r *http.Request) {
	var input loginStartRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := models.NormalizeEmail(input.Email)
	if len(email) > maxEmailLength || !models.ValidEmail(email) {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	const invalidCredentials = "invalid email or password"
	now := a.now()
	user, err := a.repo.UserByEmail(email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			models.DummyPasswordVerify(input.Password)
			writeError(w, http.StatusUnauthorized, invalidCredentials)
			return
		}
		writeRepositoryError(w, err)
		return
	}
	if user.LockedUntil.After(now) {
		writeError(w, http.StatusTooManyRequests, "too many failed attempts; try again later")
		return
	}
	match, err := models.VerifyPassword(user.PasswordHash, input.Password)
	if err != nil || !match {
		user.PasswordAttempt++
		if user.PasswordAttempt >= passwordMaxAttempts {
			user.PasswordAttempt = 0
			user.LockedUntil = now.Add(passwordLockout)
		}
		user.UpdatedAt = now
		if _, updateErr := a.repo.UpdateUser(user); updateErr != nil {
			writeRepositoryError(w, updateErr)
			return
		}
		writeError(w, http.StatusUnauthorized, invalidCredentials)
		return
	}
	if user.PasswordAttempt > 0 || !user.LockedUntil.IsZero() {
		user.PasswordAttempt, user.LockedUntil = 0, time.Time{}
		user.UpdatedAt = now
		if _, updateErr := a.repo.UpdateUser(user); updateErr != nil {
			writeRepositoryError(w, updateErr)
			return
		}
	}

	if existing, err := a.repo.LoginCode(email); err == nil {
		// The issue time is recovered from the stored expiry, so the cooldown
		// follows the API clock rather than the repository's wall clock.
		if issuedAt := existing.ExpiresAt.Add(-loginCodeTTL); now.Sub(issuedAt) < loginCodeCooldown {
			writeError(w, http.StatusTooManyRequests, "please wait before requesting another code")
			return
		}
	} else if !errors.Is(err, repository.ErrNotFound) {
		writeRepositoryError(w, err)
		return
	}
	code, err := newLoginCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate a login code")
		return
	}
	if err := a.repo.CreateLoginCode(email, models.LoginCode{CodeHash: models.HashToken(code), ExpiresAt: now.Add(loginCodeTTL)}); err != nil {
		writeRepositoryError(w, err)
		return
	}
	if err := a.sender.SendCode(r.Context(), email, code); err != nil {
		writeError(w, http.StatusBadGateway, "could not deliver the login code email")
		return
	}
	writeJSON(w, http.StatusAccepted, loginStartResponse{
		ExpiresIn:   int(loginCodeTTL / time.Second),
		ResendAfter: int(loginCodeCooldown / time.Second),
	})
}

// newLoginCode draws a uniform six-digit code from crypto/rand.
func newLoginCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

type loginVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type sessionIssued struct {
	SessionToken string      `json:"session_token"`
	User         models.User `json:"user"`
	ExpiresIn    int         `json:"expires_in"`
}

// verifyLogin exchanges an emailed code for a session token. Codes are single-use,
// capped at loginCodeMaxAttempts wrong guesses, and expire absolutely.
func (a *API) verifyLogin(w http.ResponseWriter, r *http.Request) {
	var input loginVerifyRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := models.NormalizeEmail(input.Email)
	code := strings.TrimSpace(input.Code)
	if len(email) > maxEmailLength || !models.ValidEmail(email) {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	user, err := a.repo.UserByEmail(email)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	stored, err := a.repo.LoginCode(email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "request a login code first")
			return
		}
		writeRepositoryError(w, err)
		return
	}
	now := a.now()
	if !stored.ExpiresAt.After(now) {
		_ = a.repo.DeleteLoginCode(email)
		writeError(w, http.StatusGone, "login code expired, request a new one")
		return
	}
	if !constantTimeMatch(stored.CodeHash, models.HashToken(code)) {
		stored.Attempts++
		if err := a.repo.SaveLoginCode(email, stored); err != nil {
			writeRepositoryError(w, err)
			return
		}
		if stored.Attempts >= loginCodeMaxAttempts {
			// Burn the code once guessing is exhausted; the account is untouched.
			_ = a.repo.DeleteLoginCode(email)
		}
		writeError(w, http.StatusUnauthorized, "incorrect code")
		return
	}
	if err := a.repo.DeleteLoginCode(email); err != nil {
		writeRepositoryError(w, err)
		return
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start a session")
		return
	}
	session := models.Session{UserID: user.ID, TokenHash: hash, ExpiresAt: now.Add(sessionTTL)}
	if err := a.repo.CreateSession(session); err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionIssued{SessionToken: token, User: user, ExpiresIn: int(sessionTTL / time.Second)})
}

// requireSession resolves the caller's account from a Bearer session token.
// Sessions expire absolutely; there is no sliding renewal.
func (a *API) requireSession(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	token := bearerToken(r)
	if token == "" || !validToken(token) {
		a.writeSessionUnauthorized(w)
		return models.User{}, false
	}
	session, err := a.repo.SessionByHash(models.HashToken(token))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			a.writeSessionUnauthorized(w)
			return models.User{}, false
		}
		writeRepositoryError(w, err)
		return models.User{}, false
	}
	if !session.ExpiresAt.After(a.now()) {
		_ = a.repo.DeleteSession(session.TokenHash)
		a.writeSessionUnauthorized(w)
		return models.User{}, false
	}
	user, err := a.repo.UserByID(session.UserID)
	if err != nil {
		// A session whose account vanished is not usable, so treat it like any
		// other expired session rather than erroring on stale state.
		a.writeSessionUnauthorized(w)
		return models.User{}, false
	}
	return user, true
}

func (a *API) writeSessionUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "a valid session token is required")
}

func (a *API) currentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireSession(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" || !validToken(token) {
		a.writeSessionUnauthorized(w)
		return
	}
	hash := models.HashToken(token)
	if _, err := a.repo.SessionByHash(hash); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeRepositoryError(w, err)
		return
	}
	if err := a.repo.DeleteSession(hash); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
