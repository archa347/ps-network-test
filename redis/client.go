package redis

import (
	"github.com/archa347/ps-network-test/config"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"os"
)

func RedisClient(config config.Config) *redis.Client {
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		log.WithError(err).Error("Unable to parse redis url")
		os.Exit(1)
	}

	opts.TLSConfig.InsecureSkipVerify = true

	return redis.NewClient(opts)
}
