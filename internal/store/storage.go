package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

var (
	ErrNotFound          = errors.New("record not found")
	ErrConflict          = errors.New("record already exists")
	ErrDuplicate         = errors.New("record with email already exists")
	ErrDuplicateEmail    = errors.New("record with email already exists")
	ErrDuplicateUsername = errors.New("record with username already exists")
	QueryTimeoutDuration = time.Second * 5
)

type Storage struct {
	Users interface {
		Create(context.Context, *sql.Tx, *models.User) error
		GetByID(context.Context, int64) (*models.User, error)
		CreateAndInvite(ctx context.Context, user *models.User, token string, invitationExp time.Duration) error
		ActivateUser(context.Context, string) error
		Delete(context.Context, int64) error
		ExistsByEmail(context.Context, string) (bool, error)
		GetByEmail(context.Context, string) (*models.User, error)
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
