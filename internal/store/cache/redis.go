package cache

import "github.com/go-redis/redis/v8"

func NewRedisClient(address, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})
}
