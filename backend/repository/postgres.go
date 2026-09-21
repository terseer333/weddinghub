package repository

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"

	"weddinghub/models"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// queryTimeout bounds every repository operation. The Repository interface has no
// context parameter, so each call gets its own deadline from the pooled connection.
const queryTimeout = 5 * time.Second

var _ Repository = (*PostgresRepository)(nil)

// PostgresRepository is the production Repository. Every mutation runs in an
// explicit transaction, and state transitions that depend on invitation state lock
// the invitation row before reading it, so concurrent acceptance or RSVP writes
// cannot interleave.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository connects to PostgreSQL, verifies connectivity, and applies
// the embedded migrations. The caller must Close the returned repository.
func NewPostgresRepository(ctx context.Context, dsn string) (*PostgresRepository, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("a PostgreSQL connection string is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := applyMigrations(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

// Close releases the connection pool.
func (r *PostgresRepository) Close() error { return r.db.Close() }

// applyMigrations runs each embedded migration once, tracked in schema_migrations.
func applyMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var applied bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		// A migration file may contain multiple statements; lib/pq sends a
		// parameterless Exec through the simple query protocol, which allows that.
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

func (r *PostgresRepository) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), queryTimeout)
}

// querier is satisfied by both *sql.DB and *sql.Tx, so the loaders can run inside
// or outside a transaction.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *PostgresRepository) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// mapError translates driver errors onto the sentinel errors the API expects.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23505": // unique_violation
			return ErrConflict
		case "23503": // foreign_key_violation
			return ErrNotFound
		case "23514", "22001", "22P02": // check_violation, string_data_right_truncation, invalid_text_representation
			return ErrInvalidStatus
		}
	}
	return err
}

const weddingColumns = `id, slug, title, partner_one, partner_two, date, status, venue, address, city, state,
	country, message, verse, dress_code, hero_image, template_id, card_config, admin_token_hash, created_at, updated_at`

// scanWedding reads one weddings row plus its nullable columns.
func scanWedding(row interface{ Scan(...any) error }) (models.Wedding, error) {
	var (
		w          models.Wedding
		date       *time.Time
		cardConfig []byte
	)
	if err := row.Scan(&w.ID, &w.Slug, &w.Title, &w.PartnerOne, &w.PartnerTwo, &date, &w.Status, &w.Venue, &w.Address,
		&w.City, &w.State, &w.Country, &w.Message, &w.Verse, &w.DressCode, &w.HeroImage, &w.TemplateID,
		&cardConfig, &w.AdminTokenHash, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return models.Wedding{}, mapError(err)
	}
	w.Date = date
	if len(cardConfig) > 0 {
		var cfg models.CardConfig
		if err := json.Unmarshal(cardConfig, &cfg); err == nil {
			w.CardConfig = &cfg
		}
	}
	return w, nil
}

// loadWedding assembles the full aggregate, including every child collection.
func loadWedding(ctx context.Context, q querier, id string) (models.Wedding, error) {
	w, err := scanWedding(q.QueryRowContext(ctx, `SELECT `+weddingColumns+` FROM weddings WHERE id = $1`, id))
	if err != nil {
		return models.Wedding{}, err
	}
	if err := loadAdmins(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadGuests(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadCommitteeMembers(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadCommitteeRoles(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadInvitations(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadEvents(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadPhotos(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadStorySections(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadAnnouncements(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadPlanningTasks(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadCommitteeChat(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadRSVPs(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	if err := loadGuestMessages(ctx, q, &w); err != nil {
		return models.Wedding{}, err
	}
	return w, nil
}

func loadAdmins(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT user_id, role FROM wedding_admins WHERE wedding_id = $1 ORDER BY user_id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var admin models.Admin
		if err := rows.Scan(&admin.UserID, &admin.Role); err != nil {
			return mapError(err)
		}
		w.Admins = append(w.Admins, admin)
	}
	return mapError(rows.Err())
}

func loadGuests(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, invitation_id, name, email, phone FROM guests WHERE wedding_id = $1 ORDER BY position, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var guest models.Guest
		if err := rows.Scan(&guest.ID, &guest.InvitationID, &guest.Name, &guest.Email, &guest.Phone); err != nil {
			return mapError(err)
		}
		w.Guests = append(w.Guests, guest)
	}
	return mapError(rows.Err())
}

func loadCommitteeMembers(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, invitation_id, name, email, phone, title, role_id, joined_at FROM committee_members WHERE wedding_id = $1 ORDER BY position, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var member models.CommitteeMember
		if err := rows.Scan(&member.ID, &member.InvitationID, &member.Name, &member.Email, &member.Phone, &member.Title, &member.RoleID, &member.JoinedAt); err != nil {
			return mapError(err)
		}
		w.CommitteeMembers = append(w.CommitteeMembers, member)
	}
	return mapError(rows.Err())
}

func loadCommitteeRoles(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, name, description, is_custom, created_at FROM committee_roles WHERE wedding_id = $1 ORDER BY position, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		role := models.CommitteeRole{WeddingID: w.ID}
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.IsCustom, &role.CreatedAt); err != nil {
			return mapError(err)
		}
		w.CommitteeRoles = append(w.CommitteeRoles, role)
	}
	return mapError(rows.Err())
}

func loadInvitations(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, invitation_type, guest_name, guest_email, guest_phone, committee_title,
		max_party_size, status, token_hash, expires_at, created_at, responded_at
		FROM invitations WHERE wedding_id = $1 ORDER BY position, created_at, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var invitation models.Invitation
		if err := rows.Scan(&invitation.ID, &invitation.Type, &invitation.GuestName, &invitation.GuestEmail, &invitation.GuestPhone,
			&invitation.CommitteeTitle, &invitation.MaxPartySize, &invitation.Status, &invitation.TokenHash,
			&invitation.ExpiresAt, &invitation.CreatedAt, &invitation.RespondedAt); err != nil {
			return mapError(err)
		}
		w.Invitations = append(w.Invitations, invitation)
	}
	return mapError(rows.Err())
}

func loadEvents(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, name, description, starts_at, ends_at, venue, address, status FROM events WHERE wedding_id = $1 ORDER BY position, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var event models.Event
		if err := rows.Scan(&event.ID, &event.Name, &event.Description, &event.StartsAt, &event.EndsAt, &event.Venue, &event.Address, &event.Status); err != nil {
			return mapError(err)
		}
		w.Events = append(w.Events, event)
	}
	return mapError(rows.Err())
}

func loadPhotos(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, url, alt_text, caption, sort_order, status FROM photos WHERE wedding_id = $1 ORDER BY sort_order, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var photo models.Photo
		if err := rows.Scan(&photo.ID, &photo.URL, &photo.AltText, &photo.Caption, &photo.SortOrder, &photo.Status); err != nil {
			return mapError(err)
		}
		w.Photos = append(w.Photos, photo)
	}
	return mapError(rows.Err())
}

func loadStorySections(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, title, body, photo_url, sort_order, status FROM story_sections WHERE wedding_id = $1 ORDER BY sort_order, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var section models.StorySection
		if err := rows.Scan(&section.ID, &section.Title, &section.Body, &section.PhotoURL, &section.SortOrder, &section.Status); err != nil {
			return mapError(err)
		}
		w.StorySections = append(w.StorySections, section)
	}
	return mapError(rows.Err())
}

func loadAnnouncements(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, title, body, audience, published_at, status, author_name, created_at FROM announcements WHERE wedding_id = $1 ORDER BY position, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			announcement models.Announcement
			createdAt    *time.Time
		)
		if err := rows.Scan(&announcement.ID, &announcement.Title, &announcement.Body, &announcement.Audience, &announcement.PublishedAt,
			&announcement.Status, &announcement.AuthorName, &createdAt); err != nil {
			return mapError(err)
		}
		if createdAt != nil {
			announcement.CreatedAt = *createdAt
		}
		w.Announcements = append(w.Announcements, announcement)
	}
	return mapError(rows.Err())
}

func loadPlanningTasks(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, title, details, assigned_to, due_on, status, created_by, created_at, updated_at FROM planning_tasks WHERE wedding_id = $1 ORDER BY position, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var task models.PlanningTask
		if err := rows.Scan(&task.ID, &task.Title, &task.Details, &task.AssignedTo, &task.DueOn, &task.Status, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return mapError(err)
		}
		w.PlanningTasks = append(w.PlanningTasks, task)
	}
	return mapError(rows.Err())
}

func loadCommitteeChat(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, author_id, author_name, author_role, body, created_at FROM committee_messages WHERE wedding_id = $1 ORDER BY created_at, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		message := models.CommitteeMessage{WeddingID: w.ID}
		if err := rows.Scan(&message.ID, &message.AuthorID, &message.AuthorName, &message.AuthorRole, &message.Body, &message.CreatedAt); err != nil {
			return mapError(err)
		}
		w.CommitteeChat = append(w.CommitteeChat, message)
	}
	return mapError(rows.Err())
}

func loadRSVPs(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT invitation_id, status, party_size, dietary_notes, updated_at FROM rsvps WHERE wedding_id = $1 ORDER BY updated_at, invitation_id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var response models.RSVP
		if err := rows.Scan(&response.InvitationID, &response.Status, &response.PartySize, &response.DietaryNotes, &response.UpdatedAt); err != nil {
			return mapError(err)
		}
		w.RSVPs = append(w.RSVPs, response)
	}
	return mapError(rows.Err())
}

func loadGuestMessages(ctx context.Context, q querier, w *models.Wedding) error {
	rows, err := q.QueryContext(ctx, `SELECT id, wedding_id, invitation_id, body, created_at FROM guest_messages WHERE wedding_id = $1 ORDER BY created_at, id`, w.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		message := models.GuestMessage{WeddingID: w.ID}
		if err := rows.Scan(&message.ID, &message.WeddingID, &message.InvitationID, &message.Body, &message.CreatedAt); err != nil {
			return mapError(err)
		}
		w.GuestMessages = append(w.GuestMessages, message)
	}
	return mapError(rows.Err())
}

func ensureWedding(ctx context.Context, q querier, weddingID string) error {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM weddings WHERE id = $1)`, weddingID).Scan(&exists); err != nil {
		return mapError(err)
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func nextPosition(ctx context.Context, q querier, table, weddingID string) (int, error) {
	var position int
	query := fmt.Sprintf(`SELECT COALESCE(MAX(position), 0) + 1 FROM %s WHERE wedding_id = $1`, table)
	if err := q.QueryRowContext(ctx, query, weddingID).Scan(&position); err != nil {
		return 0, mapError(err)
	}
	return position, nil
}

func cardConfigValue(config *models.CardConfig) any {
	if config == nil {
		return nil
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil
	}
	return string(encoded)
}

func (r *PostgresRepository) CreateWedding(w models.Wedding) (models.Wedding, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO weddings (id, slug, title, partner_one, partner_two, date, status, venue,
			address, city, state, country, message, verse, dress_code, hero_image, template_id, card_config, admin_token_hash, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb,$19,$20,$21)`,
			w.ID, w.Slug, w.Title, w.PartnerOne, w.PartnerTwo, w.Date, w.Status, w.Venue, w.Address, w.City, w.State, w.Country,
			w.Message, w.Verse, w.DressCode, w.HeroImage, w.TemplateID, cardConfigValue(w.CardConfig), w.AdminTokenHash,
			w.CreatedAt, w.UpdatedAt); err != nil {
			return mapError(err)
		}
		return insertAllChildren(ctx, tx, w)
	})
	if err != nil {
		return models.Wedding{}, err
	}
	return loadWedding(ctx, r.db, w.ID)
}

func (r *PostgresRepository) ListWeddings() []models.Wedding {
	ctx, cancel := r.ctx()
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM weddings ORDER BY created_at, id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil
	}
	weddings := make([]models.Wedding, 0, len(ids))
	for _, id := range ids {
		wedding, err := loadWedding(ctx, r.db, id)
		if err != nil {
			continue
		}
		weddings = append(weddings, wedding)
	}
	return weddings
}

func (r *PostgresRepository) GetWedding(id string) (models.Wedding, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	return loadWedding(ctx, r.db, id)
}

func (r *PostgresRepository) UpdateWedding(w models.Wedding) (models.Wedding, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		var (
			currentCard    []byte
			currentHash    string
			currentCreated time.Time
		)
		if err := tx.QueryRowContext(ctx, `SELECT card_config, admin_token_hash, created_at FROM weddings WHERE id = $1`, w.ID).
			Scan(&currentCard, &currentHash, &currentCreated); err != nil {
			return mapError(err)
		}
		// Invitation capabilities, guest responses, committee state, and a missing
		// card config are preserved exactly as the in-memory repository preserved them.
		if w.CardConfig == nil && len(currentCard) > 0 {
			var cfg models.CardConfig
			if err := json.Unmarshal(currentCard, &cfg); err == nil {
				w.CardConfig = &cfg
			}
		}
		w.AdminTokenHash = currentHash
		w.CreatedAt = currentCreated
		if _, err := tx.ExecContext(ctx, `UPDATE weddings SET slug=$2, title=$3, partner_one=$4, partner_two=$5, date=$6, status=$7,
			venue=$8, address=$9, city=$10, state=$11, country=$12, message=$13, verse=$14, dress_code=$15, hero_image=$16,
			template_id=$17, card_config=$18::jsonb, updated_at=$19 WHERE id=$1`,
			w.ID, w.Slug, w.Title, w.PartnerOne, w.PartnerTwo, w.Date, w.Status, w.Venue, w.Address, w.City, w.State, w.Country,
			w.Message, w.Verse, w.DressCode, w.HeroImage, w.TemplateID, cardConfigValue(w.CardConfig), w.UpdatedAt); err != nil {
			return mapError(err)
		}
		if err := replaceWeddingAdmins(ctx, tx, w); err != nil {
			return err
		}
		return replaceContent(ctx, tx, w)
	})
	if err != nil {
		return models.Wedding{}, err
	}
	return loadWedding(ctx, r.db, w.ID)
}

func (r *PostgresRepository) DeleteWedding(id string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	result, err := r.db.ExecContext(ctx, `DELETE FROM weddings WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) WeddingByAdminHash(hash string) (models.Wedding, error) {
	if strings.TrimSpace(hash) == "" {
		return models.Wedding{}, ErrNotFound
	}
	ctx, cancel := r.ctx()
	defer cancel()
	var id string
	if err := r.db.QueryRowContext(ctx, `SELECT id FROM weddings WHERE admin_token_hash = $1`, hash).Scan(&id); err != nil {
		return models.Wedding{}, mapError(err)
	}
	return loadWedding(ctx, r.db, id)
}

func (r *PostgresRepository) AddInvitation(weddingID string, invitation models.Invitation) (models.Invitation, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := ensureWedding(ctx, tx, weddingID); err != nil {
			return err
		}
		position, err := nextPosition(ctx, tx, "invitations", weddingID)
		if err != nil {
			return err
		}
		if err := insertInvitation(ctx, tx, weddingID, invitation, position); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
	if err != nil {
		return models.Invitation{}, err
	}
	return invitation, nil
}

func invitationByHash(ctx context.Context, q querier, hash string) (string, models.Invitation, error) {
	var (
		weddingID  string
		invitation models.Invitation
	)
	err := q.QueryRowContext(ctx, `SELECT wedding_id, id, invitation_type, guest_name, guest_email, guest_phone, committee_title,
		max_party_size, status, token_hash, expires_at, created_at, responded_at FROM invitations WHERE token_hash = $1`, hash).
		Scan(&weddingID, &invitation.ID, &invitation.Type, &invitation.GuestName, &invitation.GuestEmail, &invitation.GuestPhone,
			&invitation.CommitteeTitle, &invitation.MaxPartySize, &invitation.Status, &invitation.TokenHash,
			&invitation.ExpiresAt, &invitation.CreatedAt, &invitation.RespondedAt)
	if err != nil {
		return "", models.Invitation{}, mapError(err)
	}
	return weddingID, invitation, nil
}

func (r *PostgresRepository) InvitationByHash(hash string) (models.Wedding, models.Invitation, error) {
	if hash == "" {
		return models.Wedding{}, models.Invitation{}, ErrNotFound
	}
	ctx, cancel := r.ctx()
	defer cancel()
	weddingID, invitation, err := invitationByHash(ctx, r.db, hash)
	if err != nil {
		return models.Wedding{}, models.Invitation{}, err
	}
	if invitation.ExpiresAt != nil && time.Now().UTC().After(*invitation.ExpiresAt) {
		return models.Wedding{}, models.Invitation{}, ErrExpired
	}
	w, err := loadWedding(ctx, r.db, weddingID)
	if err != nil {
		return models.Wedding{}, models.Invitation{}, err
	}
	return w, invitation, nil
}

func (r *PostgresRepository) RespondToInvitation(hash string, status models.InvitationStatus, at time.Time) (models.Wedding, models.Invitation, error) {
	if status != models.InvitationAccepted && status != models.InvitationDeclined {
		return models.Wedding{}, models.Invitation{}, ErrInvalidStatus
	}
	ctx, cancel := r.ctx()
	defer cancel()
	var (
		weddingID    string
		invitationID string
	)
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		var (
			invitationType models.InvitationType
			guestName      string
			guestEmail     string
			guestPhone     string
			committeeTitle string
			currentStatus  models.InvitationStatus
			expiresAt      *time.Time
		)
		if err := tx.QueryRowContext(ctx, `SELECT id, wedding_id, invitation_type, guest_name, guest_email, guest_phone, committee_title,
			status, expires_at FROM invitations WHERE token_hash = $1 FOR UPDATE`, hash).
			Scan(&invitationID, &weddingID, &invitationType, &guestName, &guestEmail, &guestPhone, &committeeTitle,
				&currentStatus, &expiresAt); err != nil {
			return mapError(err)
		}
		if expiresAt != nil && at.After(*expiresAt) {
			return ErrExpired
		}
		if currentStatus != models.InvitationPending && currentStatus != status {
			return ErrInvalidStatus
		}
		if _, err := tx.ExecContext(ctx, `UPDATE invitations SET status = $2, responded_at = $3 WHERE id = $1`, invitationID, status, at); err != nil {
			return mapError(err)
		}
		// Acceptance materializes wedding-scoped membership for the invitation's role.
		if status == models.InvitationAccepted {
			if invitationType.Normalized() == models.InvitationCommittee {
				if err := materializeCommitteeMember(ctx, tx, weddingID, invitationID, guestName, guestEmail, guestPhone, committeeTitle, at); err != nil {
					return err
				}
			} else if err := materializeGuest(ctx, tx, weddingID, invitationID, guestName, guestEmail, guestPhone); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, at)
		return mapError(err)
	})
	if err != nil {
		return models.Wedding{}, models.Invitation{}, err
	}
	w, err := loadWedding(ctx, r.db, weddingID)
	if err != nil {
		return models.Wedding{}, models.Invitation{}, err
	}
	_, invitation, err := invitationByHash(ctx, r.db, hash)
	if err != nil {
		return models.Wedding{}, models.Invitation{}, err
	}
	return w, invitation, nil
}

func materializeGuest(ctx context.Context, tx *sql.Tx, weddingID, invitationID, name, email, phone string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM guests WHERE wedding_id = $1 AND invitation_id = $2)`, weddingID, invitationID).Scan(&exists); err != nil {
		return mapError(err)
	}
	if exists {
		return nil
	}
	guestID, err := models.NewID()
	if err != nil {
		return err
	}
	position, err := nextPosition(ctx, tx, "guests", weddingID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO guests (id, wedding_id, invitation_id, name, email, phone, position) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		guestID, weddingID, invitationID, name, email, phone, position)
	return mapError(err)
}

func materializeCommitteeMember(ctx context.Context, tx *sql.Tx, weddingID, invitationID, name, email, phone, title string, joinedAt time.Time) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM committee_members WHERE wedding_id = $1 AND invitation_id = $2)`, weddingID, invitationID).Scan(&exists); err != nil {
		return mapError(err)
	}
	if exists {
		return nil
	}
	memberID, err := models.NewID()
	if err != nil {
		return err
	}
	position, err := nextPosition(ctx, tx, "committee_members", weddingID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO committee_members (id, wedding_id, invitation_id, name, email, phone, title, role_id, joined_at, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'',$8,$9)`,
		memberID, weddingID, invitationID, name, email, phone, title, joinedAt, position)
	return mapError(err)
}

func (r *PostgresRepository) UpdateRSVP(hash string, response models.RSVP) (models.Wedding, models.RSVP, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	var weddingID string
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		var (
			invitationID  string
			currentStatus models.InvitationStatus
			expiresAt     *time.Time
		)
		if err := tx.QueryRowContext(ctx, `SELECT id, wedding_id, status, expires_at FROM invitations WHERE token_hash = $1 FOR UPDATE`, hash).
			Scan(&invitationID, &weddingID, &currentStatus, &expiresAt); err != nil {
			return mapError(err)
		}
		if expiresAt != nil && response.UpdatedAt.After(*expiresAt) {
			return ErrExpired
		}
		if currentStatus != models.InvitationAccepted {
			return ErrInvalidStatus
		}
		response.InvitationID = invitationID
		if _, err := tx.ExecContext(ctx, `INSERT INTO rsvps (invitation_id, wedding_id, status, party_size, dietary_notes, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (invitation_id) DO UPDATE SET status = EXCLUDED.status, party_size = EXCLUDED.party_size,
			dietary_notes = EXCLUDED.dietary_notes, updated_at = EXCLUDED.updated_at`,
			response.InvitationID, weddingID, response.Status, response.PartySize, response.DietaryNotes, response.UpdatedAt); err != nil {
			return mapError(err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, response.UpdatedAt)
		return mapError(err)
	})
	if err != nil {
		return models.Wedding{}, models.RSVP{}, err
	}
	w, err := loadWedding(ctx, r.db, weddingID)
	if err != nil {
		return models.Wedding{}, models.RSVP{}, err
	}
	return w, response, nil
}

func (r *PostgresRepository) AddGuestMessage(hash string, message models.GuestMessage) (models.Wedding, models.GuestMessage, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	var weddingID string
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		var (
			invitationID  string
			currentStatus models.InvitationStatus
			expiresAt     *time.Time
		)
		if err := tx.QueryRowContext(ctx, `SELECT id, wedding_id, status, expires_at FROM invitations WHERE token_hash = $1 FOR UPDATE`, hash).
			Scan(&invitationID, &weddingID, &currentStatus, &expiresAt); err != nil {
			return mapError(err)
		}
		if expiresAt != nil && message.CreatedAt.After(*expiresAt) {
			return ErrExpired
		}
		if currentStatus != models.InvitationAccepted {
			return ErrInvalidStatus
		}
		message.WeddingID = weddingID
		message.InvitationID = invitationID
		if _, err := tx.ExecContext(ctx, `INSERT INTO guest_messages (id, wedding_id, invitation_id, body, created_at) VALUES ($1,$2,$3,$4,$5)`,
			message.ID, weddingID, invitationID, message.Body, message.CreatedAt); err != nil {
			return mapError(err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, message.CreatedAt)
		return mapError(err)
	})
	if err != nil {
		return models.Wedding{}, models.GuestMessage{}, err
	}
	w, err := loadWedding(ctx, r.db, weddingID)
	if err != nil {
		return models.Wedding{}, models.GuestMessage{}, err
	}
	return w, message, nil
}

func (r *PostgresRepository) AddCommitteeMessage(weddingID string, message models.CommitteeMessage) (models.CommitteeMessage, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := ensureWedding(ctx, tx, weddingID); err != nil {
			return err
		}
		message.WeddingID = weddingID
		if _, err := tx.ExecContext(ctx, `INSERT INTO committee_messages (id, wedding_id, author_id, author_name, author_role, body, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			message.ID, weddingID, message.AuthorID, message.AuthorName, message.AuthorRole, message.Body, message.CreatedAt); err != nil {
			return mapError(err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, message.CreatedAt)
		return mapError(err)
	})
	if err != nil {
		return models.CommitteeMessage{}, err
	}
	return message, nil
}

func (r *PostgresRepository) CommitteeMessages(weddingID string, since time.Time) ([]models.CommitteeMessage, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	if err := ensureWedding(ctx, r.db, weddingID); err != nil {
		return nil, err
	}
	query := `SELECT id, wedding_id, author_id, author_name, author_role, body, created_at FROM committee_messages WHERE wedding_id = $1`
	args := []any{weddingID}
	if !since.IsZero() {
		query += ` AND created_at > $2`
		args = append(args, since)
	}
	query += ` ORDER BY created_at, id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	messages := make([]models.CommitteeMessage, 0)
	for rows.Next() {
		message := models.CommitteeMessage{WeddingID: weddingID}
		if err := rows.Scan(&message.ID, &message.WeddingID, &message.AuthorID, &message.AuthorName, &message.AuthorRole, &message.Body, &message.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		messages = append(messages, message)
	}
	return messages, mapError(rows.Err())
}

func (r *PostgresRepository) AddPlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := ensureWedding(ctx, tx, weddingID); err != nil {
			return err
		}
		position, err := nextPosition(ctx, tx, "planning_tasks", weddingID)
		if err != nil {
			return err
		}
		if err := insertPlanningTask(ctx, tx, weddingID, task, position); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, task.CreatedAt)
		return mapError(err)
	})
	if err != nil {
		return models.PlanningTask{}, err
	}
	return task, nil
}

func (r *PostgresRepository) UpdatePlanningTask(weddingID string, task models.PlanningTask) (models.PlanningTask, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT created_by, created_at FROM planning_tasks WHERE id = $1 AND wedding_id = $2`, task.ID, weddingID).
			Scan(&task.CreatedBy, &task.CreatedAt); err != nil {
			return mapError(err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE planning_tasks SET title=$3, details=$4, assigned_to=$5, due_on=$6, status=$7, updated_at=$8
			WHERE id=$1 AND wedding_id=$2`,
			task.ID, weddingID, task.Title, task.Details, task.AssignedTo, task.DueOn, task.Status, task.UpdatedAt); err != nil {
			return mapError(err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, task.UpdatedAt)
		return mapError(err)
	})
	if err != nil {
		return models.PlanningTask{}, err
	}
	return task, nil
}

func (r *PostgresRepository) DeletePlanningTask(weddingID, taskID string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	return r.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM planning_tasks WHERE id = $1 AND wedding_id = $2`, taskID, weddingID)
		if err != nil {
			return mapError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
}

func (r *PostgresRepository) AddAnnouncement(weddingID string, announcement models.Announcement) (models.Announcement, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := ensureWedding(ctx, tx, weddingID); err != nil {
			return err
		}
		position, err := nextPosition(ctx, tx, "announcements", weddingID)
		if err != nil {
			return err
		}
		if err := insertAnnouncement(ctx, tx, weddingID, announcement, position); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
	if err != nil {
		return models.Announcement{}, err
	}
	return announcement, nil
}

func (r *PostgresRepository) UpdateAnnouncement(weddingID string, announcement models.Announcement) (models.Announcement, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE announcements SET title=$3, body=$4, audience=$5, published_at=$6, status=$7, author_name=$8
			WHERE id=$1 AND wedding_id=$2`,
			announcement.ID, weddingID, announcement.Title, announcement.Body, announcement.Audience, announcement.PublishedAt,
			announcement.Status, announcement.AuthorName)
		if err != nil {
			return mapError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
	if err != nil {
		return models.Announcement{}, err
	}
	return announcement, nil
}

func (r *PostgresRepository) DeleteAnnouncement(weddingID, announcementID string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	return r.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM announcements WHERE id = $1 AND wedding_id = $2`, announcementID, weddingID)
		if err != nil {
			return mapError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
}

func (r *PostgresRepository) UpdateCardConfig(weddingID string, config models.CardConfig) (models.CardConfig, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE weddings SET card_config = $2::jsonb, template_id = $3, updated_at = $4 WHERE id = $1`,
		weddingID, cardConfigValue(&config), config.TemplateID, time.Now().UTC())
	if err != nil {
		return models.CardConfig{}, mapError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.CardConfig{}, err
	}
	if affected == 0 {
		return models.CardConfig{}, ErrNotFound
	}
	return config, nil
}

func (r *PostgresRepository) AddCommitteeRole(weddingID string, role models.CommitteeRole) (models.CommitteeRole, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := ensureWedding(ctx, tx, weddingID); err != nil {
			return err
		}
		position, err := nextPosition(ctx, tx, "committee_roles", weddingID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO committee_roles (id, wedding_id, name, description, is_custom, created_at, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			role.ID, weddingID, role.Name, role.Description, role.IsCustom, role.CreatedAt, position); err != nil {
			return mapError(err)
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
	if err != nil {
		return models.CommitteeRole{}, err
	}
	role.WeddingID = weddingID
	return role, nil
}

func (r *PostgresRepository) DeleteCommitteeRole(weddingID, roleID string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	return r.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM committee_roles WHERE id = $1 AND wedding_id = $2`, roleID, weddingID)
		if err != nil {
			return mapError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
}

func (r *PostgresRepository) UpdateCommitteeMember(weddingID string, member models.CommitteeMember) (models.CommitteeMember, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT invitation_id, joined_at FROM committee_members WHERE id = $1 AND wedding_id = $2`, member.ID, weddingID).
			Scan(&member.InvitationID, &member.JoinedAt); err != nil {
			return mapError(err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE committee_members SET name=$3, email=$4, phone=$5, title=$6, role_id=$7
			WHERE id=$1 AND wedding_id=$2`,
			member.ID, weddingID, member.Name, member.Email, member.Phone, member.Title, member.RoleID); err != nil {
			return mapError(err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
	if err != nil {
		return models.CommitteeMember{}, err
	}
	return member, nil
}

func (r *PostgresRepository) DeleteCommitteeMember(weddingID, memberID string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	return r.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM committee_members WHERE id = $1 AND wedding_id = $2`, memberID, weddingID)
		if err != nil {
			return mapError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE weddings SET updated_at = $2 WHERE id = $1`, weddingID, time.Now().UTC())
		return mapError(err)
	})
}

func (r *PostgresRepository) CreateUser(user models.User) (models.User, error) {
	email := models.NormalizeEmail(user.Email)
	if user.ID == "" || email == "" {
		return models.User{}, ErrConflict
	}
	user.Email = email
	ctx, cancel := r.ctx()
	defer cancel()
	if _, err := r.db.ExecContext(ctx, `INSERT INTO users (id, email, display_name, role, password_hash, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		user.ID, email, user.DisplayName, user.Role, user.PasswordHash, user.CreatedAt); err != nil {
		return models.User{}, mapError(err)
	}
	return user, nil
}

func userBy(ctx context.Context, r *PostgresRepository, column, value string) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, email, display_name, role, password_hash, created_at FROM users WHERE `+column+` = $1`, value).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return models.User{}, mapError(err)
	}
	return user, nil
}

func (r *PostgresRepository) UserByEmail(email string) (models.User, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	return userBy(ctx, r, "email", models.NormalizeEmail(email))
}

func (r *PostgresRepository) UserByID(id string) (models.User, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	return userBy(ctx, r, "id", id)
}

func (r *PostgresRepository) AddSession(session models.Session) error {
	if session.TokenHash == "" || session.UserID == "" {
		return ErrConflict
	}
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES ($1,$2,$3,$4)`,
		session.TokenHash, session.UserID, session.CreatedAt, session.ExpiresAt)
	return mapError(err)
}

func (r *PostgresRepository) SessionByHash(hash string) (models.Session, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	var session models.Session
	err := r.db.QueryRowContext(ctx, `SELECT token_hash, user_id, created_at, expires_at FROM sessions WHERE token_hash = $1`, hash).
		Scan(&session.TokenHash, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		return models.Session{}, mapError(err)
	}
	if !session.ExpiresAt.IsZero() && time.Now().UTC().After(session.ExpiresAt) {
		_ = r.DeleteSession(hash)
		return models.Session{}, ErrNotFound
	}
	return session, nil
}

func (r *PostgresRepository) DeleteSession(hash string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	result, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash)
	if err != nil {
		return mapError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// insertAllChildren persists every child collection supplied on wedding creation.
func insertAllChildren(ctx context.Context, tx *sql.Tx, w models.Wedding) error {
	for _, admin := range w.Admins {
		if _, err := tx.ExecContext(ctx, `INSERT INTO wedding_admins (wedding_id, user_id, role) VALUES ($1,$2,$3)`, w.ID, admin.UserID, admin.Role); err != nil {
			return mapError(err)
		}
	}
	for i, role := range w.CommitteeRoles {
		if _, err := tx.ExecContext(ctx, `INSERT INTO committee_roles (id, wedding_id, name, description, is_custom, created_at, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, role.ID, w.ID, role.Name, role.Description, role.IsCustom, role.CreatedAt, i); err != nil {
			return mapError(err)
		}
	}
	for i, guest := range w.Guests {
		if _, err := tx.ExecContext(ctx, `INSERT INTO guests (id, wedding_id, invitation_id, name, email, phone, position) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			guest.ID, w.ID, guest.InvitationID, guest.Name, guest.Email, guest.Phone, i); err != nil {
			return mapError(err)
		}
	}
	for i, member := range w.CommitteeMembers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO committee_members (id, wedding_id, invitation_id, name, email, phone, title, role_id, joined_at, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			member.ID, w.ID, member.InvitationID, member.Name, member.Email, member.Phone, member.Title, member.RoleID, member.JoinedAt, i); err != nil {
			return mapError(err)
		}
	}
	for i, invitation := range w.Invitations {
		if err := insertInvitation(ctx, tx, w.ID, invitation, i); err != nil {
			return err
		}
	}
	if err := insertContent(ctx, tx, w); err != nil {
		return err
	}
	for i, task := range w.PlanningTasks {
		if err := insertPlanningTask(ctx, tx, w.ID, task, i); err != nil {
			return err
		}
	}
	for _, message := range w.CommitteeChat {
		if _, err := tx.ExecContext(ctx, `INSERT INTO committee_messages (id, wedding_id, author_id, author_name, author_role, body, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			message.ID, w.ID, message.AuthorID, message.AuthorName, message.AuthorRole, message.Body, message.CreatedAt); err != nil {
			return mapError(err)
		}
	}
	for _, response := range w.RSVPs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO rsvps (invitation_id, wedding_id, status, party_size, dietary_notes, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			response.InvitationID, w.ID, response.Status, response.PartySize, response.DietaryNotes, response.UpdatedAt); err != nil {
			return mapError(err)
		}
	}
	for _, message := range w.GuestMessages {
		if _, err := tx.ExecContext(ctx, `INSERT INTO guest_messages (id, wedding_id, invitation_id, body, created_at) VALUES ($1,$2,$3,$4,$5)`,
			message.ID, w.ID, message.InvitationID, message.Body, message.CreatedAt); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func insertInvitation(ctx context.Context, tx *sql.Tx, weddingID string, invitation models.Invitation, position int) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO invitations (id, wedding_id, invitation_type, guest_name, guest_email, guest_phone,
		committee_title, max_party_size, status, token_hash, expires_at, created_at, responded_at, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		invitation.ID, weddingID, invitation.Type, invitation.GuestName, invitation.GuestEmail, invitation.GuestPhone,
		invitation.CommitteeTitle, invitation.MaxPartySize, invitation.Status, invitation.TokenHash,
		invitation.ExpiresAt, invitation.CreatedAt, invitation.RespondedAt, position)
	return mapError(err)
}

// insertContent writes the content collections that UpdateWedding replaces wholesale.
func insertContent(ctx context.Context, tx *sql.Tx, w models.Wedding) error {
	for i, event := range w.Events {
		if _, err := tx.ExecContext(ctx, `INSERT INTO events (id, wedding_id, name, description, starts_at, ends_at, venue, address, status, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			event.ID, w.ID, event.Name, event.Description, event.StartsAt, event.EndsAt, event.Venue, event.Address, event.Status, i); err != nil {
			return mapError(err)
		}
	}
	for _, photo := range w.Photos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO photos (id, wedding_id, url, alt_text, caption, sort_order, status) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			photo.ID, w.ID, photo.URL, photo.AltText, photo.Caption, photo.SortOrder, photo.Status); err != nil {
			return mapError(err)
		}
	}
	for _, section := range w.StorySections {
		if _, err := tx.ExecContext(ctx, `INSERT INTO story_sections (id, wedding_id, title, body, photo_url, sort_order, status) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			section.ID, w.ID, section.Title, section.Body, section.PhotoURL, section.SortOrder, section.Status); err != nil {
			return mapError(err)
		}
	}
	for i, announcement := range w.Announcements {
		if err := insertAnnouncement(ctx, tx, w.ID, announcement, i); err != nil {
			return err
		}
	}
	return nil
}

func insertAnnouncement(ctx context.Context, tx *sql.Tx, weddingID string, announcement models.Announcement, position int) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO announcements (id, wedding_id, title, body, audience, published_at, status, author_name, created_at, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		announcement.ID, weddingID, announcement.Title, announcement.Body, announcement.Audience, announcement.PublishedAt,
		announcement.Status, announcement.AuthorName, announcement.CreatedAt, position)
	return mapError(err)
}

func insertPlanningTask(ctx context.Context, tx *sql.Tx, weddingID string, task models.PlanningTask, position int) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO planning_tasks (id, wedding_id, title, details, assigned_to, due_on, status, created_by, created_at, updated_at, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		task.ID, weddingID, task.Title, task.Details, task.AssignedTo, task.DueOn, task.Status, task.CreatedBy, task.CreatedAt, task.UpdatedAt, position)
	return mapError(err)
}

// replaceWeddingAdmins swaps the admin roster, matching UpdateWedding's semantics.
func replaceWeddingAdmins(ctx context.Context, tx *sql.Tx, w models.Wedding) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM wedding_admins WHERE wedding_id = $1`, w.ID); err != nil {
		return mapError(err)
	}
	for _, admin := range w.Admins {
		if _, err := tx.ExecContext(ctx, `INSERT INTO wedding_admins (wedding_id, user_id, role) VALUES ($1,$2,$3)`, w.ID, admin.UserID, admin.Role); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// replaceContent swaps the four content collections UpdateWedding replaces.
func replaceContent(ctx context.Context, tx *sql.Tx, w models.Wedding) error {
	for _, table := range []string{"events", "photos", "story_sections", "announcements"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE wedding_id = $1`, table), w.ID); err != nil {
			return mapError(err)
		}
	}
	return insertContent(ctx, tx, w)
}
