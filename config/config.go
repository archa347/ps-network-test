package config

import (
	"fmt"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DynoID             string
	RedisURL           string
	LivenessIntervalMS int
	LivenessTimeoutMS  int
	AppName            string
	PrivateCheckCron   string
	DMZCheckCron       string
	NATCheckCron       string
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
	intvstring, set := os.LookupEnv("LIVENESS_INTERVAL_MS")
	if !set {
		intvstring = "10000"
	}
	cfg.LivenessIntervalMS, err = strconv.Atoi(intvstring)
	if err != nil {
		log.Error("Invalid LIVENESS_INTERVAL.  Must be integer")
		os.Exit(1)
	}

	timeoutstr, _ := os.LookupEnv("LIVENESS_TIMEOUT_MS")
	cfg.LivenessTimeoutMS, err = strconv.Atoi(timeoutstr)
	if err != nil {
		cfg.LivenessTimeoutMS = 3 * cfg.LivenessIntervalMS
	}

	cfg.AppName, set = os.LookupEnv("HEROKU_DNS_APP_NAME")
	if !set {
		cfg.AppName = fmt.Sprintf("localhost:%v", cfg.Port)
	}

	cfg.DMZCheckCron, set = os.LookupEnv("DMZ_CHECK_CRON")
	if !set {
		cfg.DMZCheckCron = "0/15 * * * * *"
	}

	cfg.NATCheckCron, set = os.LookupEnv("NAT_CHECK_CRON")
	if !set {
		cfg.NATCheckCron = "0/15 * * * * *"
	}

	cfg.PrivateCheckCron, set = os.LookupEnv("NAT_CHECK_CRON")
	if !set {
		cfg.PrivateCheckCron = "0 * * * * *"
	}

	return cfg
}
