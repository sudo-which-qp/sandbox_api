package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

type UserStore struct {
	db *sql.DB
}

func (storage *UserStore) CreateAndInvite(ctx context.Context, user *models.User, token string, invitationExp time.Duration) error {
	// transaction wrapper
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		// create the user
		if err := storage.Create(ctx, tx, user); err != nil {
			return err
		}

		// create user invitation
		if err := storage.createUserInvitation(ctx, tx, token, invitationExp, user.ID); err != nil {
			return err
		}

		return nil
	})
}

func (storage *UserStore) Create(ctx context.Context, tx *sql.Tx, user *models.User) error {
	query := `
    INSERT INTO users (username, email, normalized_email, password, role_id) 
    VALUES (?, ?, ?, ?, (SELECT id FROM roles WHERE name = ?))`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	user.NormalizedEmail = storage.normalizeEmail(user.Email)

	role := user.Role.Name
	if role == "" {
		role = "user"
	}

	result, err := tx.ExecContext(
		ctx,
		query,
		user.Username,
		user.Email,
		user.NormalizedEmail,
		user.Password.Hash,
		role,
	)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			if strings.Contains(mysqlErr.Message, "users.email") {
				return ErrDuplicateEmail
			}
			if strings.Contains(mysqlErr.Message, "users.normalized_email") {
				return ErrDuplicateEmail
			}
			if strings.Contains(mysqlErr.Message, "users.username") {
				return ErrDuplicateUsername
			}
		}
		return err
	}

	// Get the auto-generated ID
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id

	// Get the timestamps with a separate query
	err = tx.QueryRowContext(
		ctx,
		`SELECT created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	normalizedEmail := storage.normalizeEmail(email)

	query := `SELECT 1 FROM users WHERE normalized_email = ? LIMIT 1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists int
	err := storage.db.QueryRowContext(ctx, query, normalizedEmail).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return exists == 1, nil
}

func (storage *UserStore) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `
		SELECT 
			users.id, 
			users.username, 
			users.email, 
			users.is_active, 
			users.role_id, 
			users.created_at, 
			users.updated_at, 
			roles.id AS role_id, 
			roles.name AS role_name, 
			roles.level AS role_level, 
			roles.description AS role_description
		FROM users
		JOIN roles ON users.role_id = roles.id 
		WHERE users.id = ? AND users.is_active = true`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	row := storage.db.QueryRowContext(ctx, query, id)

	user := &models.User{}
	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.IsActive,
		&user.RoleID,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role.ID,
		&user.Role.Name,
		&user.Role.Level,
		&user.Role.Description,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return user, nil
}

func (storage *UserStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	normalizedEmail := storage.normalizeEmail(email)

	query := `
	SELECT id, username, email, password, created_at, updated_at FROM users WHERE normalized_email = ? AND is_active = true`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	row := storage.db.QueryRowContext(ctx, query, normalizedEmail)

	user := &models.User{}
	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password.Hash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return user, nil
}

func (storage *UserStore) ActivateUser(ctx context.Context, token string) error {

	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		// 1. find the user that this token belongs to
		user, err := storage.gerUserFromInvitation(ctx, tx, token)
		if err != nil {
			return err
		}

		// 2. update the user
		user.IsActive = true
		if err := storage.update(ctx, tx, user); err != nil {
			return err
		}

		// 3. clean the invitations
		if err := storage.deleteUserInvitations(ctx, tx, user.ID); err != nil {
			return err
		}

		return nil
	})
}

func (storage *UserStore) Delete(ctx context.Context, userID int64) error {
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		if err := storage.delete(ctx, tx, userID); err != nil {
			return err
		}

		if err := storage.deleteUserInvitations(ctx, tx, userID); err != nil {
			return err
		}

		return nil
	})
}

//================== Private methods ======================//

func (storage *UserStore) createUserInvitation(ctx context.Context, tx *sql.Tx, token string, invitationExp time.Duration, userID int64) error {
	query := `INSERT INTO user_invitations (token, user_id, expires_at) VALUES (?, ?, ?)`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, token, userID, time.Now().Add(invitationExp))
	if err != nil {
		return err
	}

	return err
}

func (storage *UserStore) gerUserFromInvitation(ctx context.Context, tx *sql.Tx, token string) (*models.User, error) {
	query := `
		SELECT u.id, u.username, u.email, u.created_at, u.is_active
		FROM users u
		JOIN user_invitations ui ON u.id = ui.user_id
		WHERE ui.token = ? AND ui.expires_at >= ?
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	hash := sha256.Sum256([]byte(token))
	hashToken := hex.EncodeToString(hash[:])

	fmt.Printf("Token: %v", hashToken)

	row := tx.QueryRowContext(ctx, query, hashToken, time.Now())

	user := &models.User{}
	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
		&user.IsActive,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return user, nil

}

func (storage *UserStore) update(ctx context.Context, tx *sql.Tx, user *models.User) error {
	query := `UPDATE users
			  SET username = ?, email = ?, is_active = ?
              WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, user.Username, user.Email, user.IsActive, user.ID)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) deleteUserInvitations(ctx context.Context, tx *sql.Tx, userID int64) error {
	query := `DELETE FROM user_invitations WHERE user_id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, userID)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) delete(ctx context.Context, tx *sql.Tx, userID int64) error {
	query := `DELETE FROM users WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, userID)

	if err != nil {
		return err
	}

	return nil
}

// Helper function to normalize email addresses
func (storage *UserStore) normalizeEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email // Not a valid email format
	}

	username := parts[0]
	domain := parts[1]

	// Remove everything after the first "+" in the username part
	if plusIndex := strings.Index(username, "+"); plusIndex != -1 {
		username = username[:plusIndex]
	}

	return strings.ToLower(username + "@" + domain)
}
