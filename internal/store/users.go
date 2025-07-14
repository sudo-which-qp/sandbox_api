package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"godsendjoseph.dev/sandbox-api/internal/models"
	"strings"
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

func (storage *UserStore) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `
		SELECT 
			users.id, 
			users.first_name, 
			users.last_name,
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
		WHERE users.id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	row := storage.db.QueryRowContext(ctx, query, id)

	user := &models.User{}
	err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
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

	if !user.IsActive {
		return nil, ErrAccountNotVerified
	}

	return user, nil
}

func (storage *UserStore) GetByEmail(ctx context.Context, email string, isAuth bool) (*models.User, error) {
	normalizedEmail := normalizeEmail(email)

	query := `
    SELECT 
    u.id, u.username, u.email, u.password, u.otp_code, u.otp_expires_at, u.is_active, u.created_at, u.updated_at, 
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
		&user.OtpCode,
		&user.OtpExp,
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

	if !user.IsActive && isAuth == true {
		return nil, ErrAccountNotVerified
	}

	return user, nil
}

func (storage *UserStore) UpdateUserProfile(ctx context.Context, user *models.User) error {
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		return storage.updateQuery(ctx, tx, user)
	})
}

func (storage *UserStore) VerifyEmail(ctx context.Context, userId int64) error {
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		return storage.verifyEmailQuery(ctx, tx, userId)
	})
}

func (storage *UserStore) UpdateOTPCode(ctx context.Context, user *models.User, otpCode string, otpExp string) error {
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		return storage.updateOTPQuery(ctx, tx, user, otpCode, otpExp)
	})
}

func (storage *UserStore) ResetPassword(ctx context.Context, user *models.User) error {
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		return storage.resetPasswordQuery(ctx, tx, user)
	})
}

func (storage *UserStore) Delete(ctx context.Context, userID int64) error {
	return withTx(ctx, storage.db, func(tx *sql.Tx) error {
		if err := storage.deleteQuery(ctx, tx, userID); err != nil {
			return err
		}
		return nil
	})
}

// ================== Private methods ======================//
func (storage *UserStore) updateQuery(ctx context.Context, tx *sql.Tx, user *models.User) error {
	query := `UPDATE users
			  SET first_name = ?, last_name = ?
			  WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, user.FirstName, user.LastName, user.ID)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) resetPasswordQuery(ctx context.Context, tx *sql.Tx, user *models.User) error {
	query := `UPDATE users
			  SET password = ?, otp_code = ?
			  WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, user.Password.Hash, "", user.ID)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) verifyEmailQuery(ctx context.Context, tx *sql.Tx, userID int64) error {
	query := `UPDATE users
			  SET is_active = ?, otp_code = ?
			  WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, true, "", userID)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) updateOTPQuery(ctx context.Context, tx *sql.Tx, user *models.User, otpCode string, otpExp string) error {
	query := `UPDATE users
			  SET otp_code = ?, otp_expires_at = ?
			  WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, otpCode, otpExp, user.ID)

	if err != nil {
		return err
	}

	return nil
}

func (storage *UserStore) deleteQuery(ctx context.Context, tx *sql.Tx, userID int64) error {
	query := `DELETE FROM users WHERE id = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, userID)

	if err != nil {
		return err
	}

	return nil
}
