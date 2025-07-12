package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

var (
	ErrNotFound           = errors.New("record not found")
	ErrConflict           = errors.New("record already exists")
	ErrDuplicate          = errors.New("record with email already exists")
	ErrDuplicateEmail     = errors.New("record with email already exists")
	ErrDuplicateUsername  = errors.New("record with username already exists")
	ErrAccountNotVerified = errors.New("account is not verified")
	QueryTimeoutDuration  = time.Second * 5
)

type Storage struct {
	Users interface {
		Create(context.Context, *sql.Tx, *models.User) error
		GetByID(context.Context, int64) (*models.User, error)
		CreateUserTx(context.Context, *models.User) error
		Delete(context.Context, int64) error
		GetByEmail(context.Context, string) (*models.User, error)
		UpdateOTPCode(context context.Context, user *models.User, otpCode string, otpExpiresAt string) error
		VerifyEmail(context.Context, int64) error
	}
	Roles interface {
		GetByName(context.Context, string) (*models.Role, error)
	}
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Users: &UserStore{db},
		Roles: &RoleStore{db},
	}
}

func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		if err := tx.Rollback(); err != nil {
			return err
		}
		return err
	}

	return tx.Commit()
}

func normalizeEmail(email string) string {
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
