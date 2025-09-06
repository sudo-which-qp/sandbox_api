package cache

import (
	"context"

	"github.com/go-redis/redis/v8"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

type Storage struct {
	Users interface {
		Get(context.Context, int64) (*models.User, error)
		Set(context.Context, *models.User) error
		Update(context.Context, *models.User) error
		Delete(context.Context, int64) error
	}
}

func NewRedisStorage(rdb *redis.Client) Storage {
	return Storage{
		Users: &UserStore{rdb: rdb},
	}
}
