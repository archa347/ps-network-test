package redis

import (
	"github.com/archa347/ps-network-test/config"
	"github.com/go-redis/redis/v8"
)

func RedisClient(config config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: config.RedisURL,
	})
}
