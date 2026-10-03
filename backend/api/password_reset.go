package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"weddinghub/models"
	"weddinghub/repository"
)

const (
	passwordResetTTL         = 30 * time.Minute
	passwordResetLimitWindow = time.Hour
	passwordResetIPLimit     = 5
	passwordResetEmailLimit  = 3
	passwordResetReply       = "If an account exists for that email, a password reset link will be sent. Check your inbox and spam folder."
)

type passwordResetLimiter struct {
	mu      sync.Mutex
	entries map[string]resetLimitEntry
}

type resetLimitEntry struct {
	started time.Time
	count   int
}

func newPasswordResetLimiter() *passwordResetLimiter {
	return &passwordResetLimiter{entries: make(map[string]resetLimitEntry)}
}

func (l *passwordResetLimiter) allow(ip, email string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.entries {
		if now.Sub(entry.started) >= passwordResetLimitWindow {
			delete(l.entries, key)
		}
	}
	emailDigest := sha256.Sum256([]byte(email))
	keys := []struct {
		key   string
		limit int
	}{
		{key: "ip:" + ip, limit: passwordResetIPLimit},
		{key: "email:" + hex.EncodeToString(emailDigest[:]), limit: passwordResetEmailLimit},
	}
	if len(l.entries) > 10000 {
		return false
	}
	for _, item := range keys {
		entry := l.entries[item.key]
		if entry.started.IsZero() || now.Sub(entry.started) >= passwordResetLimitWindow {
			continue
		}
		if entry.count >= item.limit {
			return false
		}
	}
	for _, item := range keys {
		entry := l.entries[item.key]
		if entry.started.IsZero() || now.Sub(entry.started) >= passwordResetLimitWindow {
			entry = resetLimitEntry{started: now}
		}
		entry.count++
		l.entries[item.key] = entry
	}
	return true
}

func passwordResetClientIP(r *http.Request) string {
	address := clientAddress(r)
	if parsed := net.ParseIP(address); parsed != nil {
		return parsed.String()
	}
	return "unknown"
}

func (a *API) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "a valid email address is required")
		return
	}
	email := models.NormalizeEmail(input.Email)
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "a valid email address is required")
		return
	}
	allowed := a.resetLimiter.allow(passwordResetClientIP(r), email, a.now())
	if a.passwordMailer != nil && allowed {
		go a.deliverPasswordReset(email)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": passwordResetReply})
}

func (a *API) deliverPasswordReset(email string) {
	user, err := a.repo.UserByEmail(email)
	if errors.Is(err, repository.ErrNotFound) {
		return
	}
	if err != nil {
		log.Printf("password reset account lookup failed")
		return
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		log.Printf("password reset token generation failed")
		return
	}
	now := a.now()
	if err := a.repo.CreatePasswordResetToken(models.PasswordResetToken{UserID: user.ID, TokenHash: hash, CreatedAt: now, ExpiresAt: now.Add(passwordResetTTL)}); err != nil {
		log.Printf("password reset token storage failed")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := a.passwordMailer.SendPasswordReset(ctx, user.Email, user.DisplayName, token); err != nil {
		log.Printf("password reset email delivery failed")
	}
}

func (a *API) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid reset request")
		return
	}
	if !validToken(input.Token) {
		writeError(w, http.StatusBadRequest, "This password reset link is invalid or has already been used.")
		return
	}
	if len(input.Password) < models.MinPasswordLength {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := models.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not secure this password")
		return
	}
	user, err := a.repo.CompletePasswordReset(models.HashToken(input.Token), hash, a.now())
	if errors.Is(err, repository.ErrResetTokenExpired) {
		writeError(w, http.StatusGone, "This password reset link has expired. Request a new one.")
		return
	}
	if errors.Is(err, repository.ErrResetTokenInvalid) || errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "This password reset link is invalid or has already been used.")
		return
	}
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if a.passwordMailer != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			if err := a.passwordMailer.SendPasswordChanged(ctx, user.Email, user.DisplayName); err != nil {
				log.Printf("password change notification delivery failed")
			}
		}()
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Your password has been reset. Sign in with your new password."})
}
