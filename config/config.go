package config

import (
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
)

type Config struct {
	Port             string
	DynoID           string
	RedisURL         string
	LivenessInterval int
}

func New() Config {
	cfg := Config{}

	err := godotenv.Load(".env")
	if err != nil {
		log.WithError(err).Info("Unable to load .env")
	}

	var set bool
	cfg.Port, set = os.LookupEnv("PORT")
	if !set {
		cfg.Port = "5000"
	}
	cfg.DynoID, set = os.LookupEnv("HEROKU_DNS_DYNO_NAME")
	if !set {
		cfg.DynoID, set = os.LookupEnv("DYNO")
		if !set {
			cfg.DynoID = "local.1"
		}
	}
	cfg.RedisURL, set = os.LookupEnv("REDIS_URL")
	if !set || cfg.RedisURL == "" {
		log.Error("REDIS_URL not found")
		os.Exit(1)
	}
	intvstring, set := os.LookupEnv("LIVENESS_INTERVAL")
	if !set {
		intvstring = "10000"
	}
	cfg.LivenessInterval, err = strconv.Atoi(intvstring)
	if err != nil {
		log.Error("Invalid LIVENESS_INTERVAL.  Must be integer")
		os.Exit(1)
	}

	return cfg
}
