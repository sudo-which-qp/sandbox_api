package store

import (
	"context"
	"database/sql"
	"errors"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

type RoleStore struct {
	db *sql.DB
}

func (storage *RoleStore) GetByName(ctx context.Context, slug string) (*models.Role, error) {
	query := `SELECT id, name, description, level FROM roles WHERE name = ?`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	row := storage.db.QueryRowContext(ctx, query, slug)

	role := &models.Role{}
	err := row.Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.Level,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return role, nil
}
