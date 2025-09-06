package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"godsendjoseph.dev/sandbox-api/internal/models"
)

type UserStore struct {
	rdb *redis.Client
}

const UserExpTime = time.Minute * 5

func (storage *UserStore) Get(ctx context.Context, userID int64) (*models.User, error) {
	if storage.rdb == nil {
		return nil, errors.New("redis client not initialized")
	}

	cacheKey := fmt.Sprintf("user-%v", userID)

	data, err := storage.rdb.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var user models.User

	if data != "" {
		err := json.Unmarshal([]byte(data), &user)
		if err != nil {
			return nil, err
		}
	}

	return &user, nil
}

func (storage *UserStore) Set(ctx context.Context, user *models.User) error {
	cacheKey := fmt.Sprintf("user-%v", user.ID)

	json, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return storage.rdb.SetEX(ctx, cacheKey, json, UserExpTime).Err()
}

func (storage *UserStore) Update(ctx context.Context, user *models.User) error {
	if storage.rdb == nil {
		return errors.New("redis client not initialized")
	}

	// Check if the key exists first
	cacheKey := fmt.Sprintf("user-%v", user.ID)
	exists, err := storage.rdb.Exists(ctx, cacheKey).Result()
	if err != nil {
		return err
	}

	if exists == 0 {
		return errors.New("user not found in cache")
	}

	// Update is essentially the same as Set - overwrite with new data
	json, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return storage.rdb.SetEX(ctx, cacheKey, json, UserExpTime).Err()
}

func (storage *UserStore) Delete(ctx context.Context, userID int64) error {
	if storage.rdb == nil {
		return errors.New("redis client not initialized")
	}

	cacheKey := fmt.Sprintf("user-%v", userID)

	result, err := storage.rdb.Del(ctx, cacheKey).Result()
	if err != nil {
		return err
	}

	// Optional: return an error if the key didn't exist
	if result == 0 {
		return errors.New("user not found in cache")
	}

	return nil
}
