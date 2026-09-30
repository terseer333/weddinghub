package api

import (
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"weddinghub/models"
	"weddinghub/repository"
)

// Admin login throttling. The limiter lives in memory, which matches this project's deployment
// shape: a single API process serves the site. A future multi-instance deployment would move the
// counters into PostgreSQL so every instance shares them.
const (
	adminLoginMaxFailures = 5
	adminLoginWindow      = 15 * time.Minute
	adminLoginLockout     = 15 * time.Minute
)

type loginAttempt struct {
	failures int
	first    time.Time
	until    time.Time
}

type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
	now      func() time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{
		attempts: make(map[string]*loginAttempt),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// loginGuardKey scopes the counter to the attempted email and the client address together, so one
// noisy address cannot lock an administrator out of their own account from elsewhere.
func loginGuardKey(email, address string) string { return email + "|" + address }

// blocked reports whether the key is locked out and how long remains.
func (g *loginGuard) blocked(key string) (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	attempt, ok := g.attempts[key]
	if !ok {
		return 0, false
	}
	now := g.now()
	if now.Before(attempt.until) {
		return attempt.until.Sub(now), true
	}
	if !attempt.until.IsZero() || now.Sub(attempt.first) > adminLoginWindow {
		delete(g.attempts, key)
	}
	return 0, false
}

// failed records a failed attempt and starts a lockout once the threshold is reached.
func (g *loginGuard) failed(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	attempt, ok := g.attempts[key]
	if !ok || now.Sub(attempt.first) > adminLoginWindow {
		attempt = &loginAttempt{first: now}
		g.attempts[key] = attempt
	}
	attempt.failures++
	if attempt.failures >= adminLoginMaxFailures {
		attempt.until = now.Add(adminLoginLockout)
	}
}

// succeeded clears the counter after a valid sign-in.
func (g *loginGuard) succeeded(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, key)
}

// clientAddress identifies the caller for throttling. A host such as Render terminates TLS in
// front of the process, so the first X-Forwarded-For entry is the browser; a direct caller has
// only RemoteAddr.
func clientAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isAdminEmail reports whether email is the configured administrator. An unset configuration
// matches nobody, so a deployment without WEDDINGHUB_ADMIN_EMAIL has no administrator at all.
func (a *API) isAdminEmail(email string) bool {
	return a.adminEmail != "" && models.NormalizeEmail(email) == a.adminEmail
}

// markAdmin sets the computed admin flag on an account for its response. The flag is derived from
// server configuration, never from anything the client sent.
func (a *API) markAdmin(user models.User) models.User {
	user.IsAdmin = a.isAdminEmail(user.Email)
	return user
}

// recordAudit appends an entry to the admin audit trail. Auditing is best-effort: a failed write
// is logged, never turned into a request error, so a legitimate action is not undone by it.
func (a *API) recordAudit(r *http.Request, entry models.AuditLog) {
	if entry.ID == "" {
		id, err := models.NewID()
		if err != nil {
			log.Printf("admin audit: could not generate identifier: %v", err)
			return
		}
		entry.ID = id
	}
	entry.CreatedAt = a.now()
	if r != nil {
		if entry.Metadata == nil {
			entry.Metadata = map[string]string{}
		}
		entry.Metadata["client_address"] = clientAddress(r)
	}
	if err := a.repo.AddAuditLog(entry); err != nil {
		log.Printf("admin audit: could not record %s: %v", entry.Action, err)
	}
}

// requireAdminUser resolves the caller's browser session and requires that the account is the
// configured administrator. Authorization is entirely server-side: a normal account session is
// refused with 403, so the dashboard cannot be reached by editing anything in the browser.
func (a *API) requireAdminUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return models.User{}, false
	}
	if !a.isAdminEmail(user.Email) {
		a.recordAudit(r, models.AuditLog{ActorID: user.ID, ActorEmail: user.Email, Action: models.AuditAdminAccessDenied})
		writeError(w, http.StatusForbidden, "this account is not the platform administrator")
		return models.User{}, false
	}
	return a.markAdmin(user), true
}

// adminLogin authenticates the administrator. It reuses the account and session tables, so there
// is one credential store rather than a second, weaker one. A valid non-administrator account is
// refused exactly like a wrong password — same status, same work — so the endpoint does not reveal
// which addresses have accounts or which one is the administrator.
func (a *API) adminLogin(w http.ResponseWriter, r *http.Request) {
	var input credentialsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := models.NormalizeEmail(input.Email)
	key := loginGuardKey(email, clientAddress(r))
	if wait, blocked := a.loginGuard.blocked(key); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "too many failed sign-in attempts; try again later")
		return
	}

	refuse := func() {
		_ = models.VerifyPassword(dummyPasswordHash, input.Password)
		a.loginGuard.failed(key)
		a.recordAudit(r, models.AuditLog{ActorEmail: email, Action: models.AuditAdminLoginFailed})
		writeError(w, http.StatusUnauthorized, "email or password is incorrect")
	}

	user, err := a.repo.UserByEmail(email)
	if err != nil || !a.isAdminEmail(user.Email) {
		refuse()
		return
	}
	if err := models.VerifyPassword(user.PasswordHash, input.Password); err != nil {
		refuse()
		return
	}
	a.loginGuard.succeeded(key)
	a.recordAudit(r, models.AuditLog{ActorID: user.ID, ActorEmail: user.Email, Action: models.AuditAdminLogin})
	a.issueSession(w, a.markAdmin(user), http.StatusOK)
}

// adminMe returns the signed-in administrator, which the dashboard uses to confirm access and
// render the account. Any other caller is refused by requireAdminUser.
func (a *API) adminMe(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// adminLogout ends the administrator's session, so the token cannot be replayed afterwards.
func (a *API) adminLogout(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	if err := a.repo.DeleteSession(models.HashToken(bearerToken(r))); err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeRepositoryError(w, err)
		return
	}
	a.recordAudit(r, models.AuditLog{ActorID: user.ID, ActorEmail: user.Email, Action: models.AuditAdminLogout})
	w.WriteHeader(http.StatusNoContent)
}
