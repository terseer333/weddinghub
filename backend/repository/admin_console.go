package repository

import (
	"strings"
	"time"

	"weddinghub/models"
)

func (r *PostgresRepository) ListUsers(search, status string) ([]models.User, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT id,email,display_name,role,password_hash,created_at,status FROM users
		WHERE ($1='' OR lower(email) LIKE '%'||lower($1)||'%' OR lower(display_name) LIKE '%'||lower($1)||'%')
		AND ($2='' OR status=$2) ORDER BY created_at DESC LIMIT 500`, strings.TrimSpace(search), status)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.PasswordHash, &user.CreatedAt, &user.Status); err != nil {
			return nil, mapError(err)
		}
		users = append(users, user)
	}
	return users, mapError(rows.Err())
}

func (r *PostgresRepository) SetUserStatus(id, status string) (models.User, error) {
	if status != "active" && status != "suspended" {
		return models.User{}, ErrInvalidStatus
	}
	ctx, cancel := r.ctx()
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.User{}, mapError(err)
	}
	defer tx.Rollback()
	var user models.User
	err = tx.QueryRowContext(ctx, `UPDATE users SET status=$2 WHERE id=$1 RETURNING id,email,display_name,role,password_hash,created_at,status`, id, status).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.PasswordHash, &user.CreatedAt, &user.Status)
	if err != nil {
		return models.User{}, mapError(err)
	}
	if status == "suspended" {
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=$1`, id); err != nil {
			return models.User{}, mapError(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return models.User{}, mapError(err)
	}
	return user, nil
}

func (r *PostgresRepository) DeleteUser(id string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return mapError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) SetUserPasswordHash(id, hash string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash=$2,status='active' WHERE id=$1`, id, hash)
	if err != nil {
		return mapError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) EnsurePlatformAdmin(email, hash string, now time.Time) error {
	email = models.NormalizeEmail(email)
	if email == "" || hash == "" {
		return ErrConflict
	}
	user, err := r.UserByEmail(email)
	if err == nil {
		return r.SetUserPasswordHash(user.ID, hash)
	}
	if err != ErrNotFound {
		return err
	}
	id, err := models.NewID()
	if err != nil {
		return err
	}
	_, err = r.CreateUser(models.User{ID: id, Email: email, DisplayName: "WeddingHub Owner", Role: models.RoleOwner, Status: "active", PasswordHash: hash, CreatedAt: now})
	return err
}

func (r *PostgresRepository) PlatformStats(adminEmail string, now time.Time) (PlatformStats, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var stats PlatformStats
	err := r.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM users WHERE lower(email)<>lower($1)),
		(SELECT count(*) FROM users WHERE lower(email)<>lower($1) AND status='active'),
		(SELECT count(*) FROM users WHERE lower(email)<>lower($1) AND status='suspended'),
		(SELECT count(*) FROM users WHERE lower(email)<>lower($1) AND created_at >= $2),
		(SELECT count(*) FROM weddings)`, models.NormalizeEmail(adminEmail), month).
		Scan(&stats.TotalUsers, &stats.ActiveUsers, &stats.SuspendedUsers, &stats.NewUsersThisMonth, &stats.TotalWeddings)
	return stats, mapError(err)
}
