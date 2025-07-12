package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"strings"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

type UserStore struct {
	db *sql.DB
}

func (storage *UserStore) CreateUserTx(ctx context.Context, user *models.User) error {
	// transaction wrapper
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		// create the user
		if err := storage.Create(ctx, tx, user); err != nil {
			return err
		}
		return nil
	})
}

func (storage *UserStore) Create(ctx context.Context, tx *sql.Tx, user *models.User) error {
	query := `
    INSERT INTO users (first_name, last_name, username, email, normalized_email, otp_code, otp_expires_at, password, role_id) 
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, (SELECT id FROM roles WHERE name = ?))`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	user.NormalizedEmail = normalizeEmail(user.Email)

	role := user.Role.Name
	if role == "" {
		role = "user"
	}

	result, err := tx.ExecContext(
		ctx,
		query,
		user.FirstName,
		user.LastName,
		user.Username,
		user.Email,
		user.NormalizedEmail,
		user.OtpCode,
		user.OtpExp,
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
	normalizedEmail := normalizeEmail(email)

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
	normalizedEmail := normalizeEmail(email)

	query := `
    SELECT 
    u.id, u.username, u.email, u.password, u.is_active, u.created_at, u.updated_at, 
    u.role_id,
    r.id, r.name, r.level, r.description
    FROM users u
    LEFT JOIN roles r ON u.role_id = r.id
    WHERE u.normalized_email = ?
`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	row := storage.db.QueryRowContext(ctx, query, normalizedEmail)

	user := &models.User{}
	var roleID sql.NullInt64
	var roleName sql.NullString
	var roleLevel sql.NullInt64
	var roleDescription sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password.Hash,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.RoleID,
		&roleID,
		&roleName,
		&roleLevel,
		&roleDescription,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	// Set role fields only if they're not NULL
	if roleID.Valid {
		user.Role.ID = roleID.Int64
		user.Role.Name = roleName.String
		user.Role.Level = int(roleLevel.Int64)
		user.Role.Description = roleDescription.String
	}

	if !user.IsActive {
		return nil, ErrAccountNotVerified
	}

	return user, nil
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
