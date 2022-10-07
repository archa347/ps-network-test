package liveness

import (
	"context"
	"fmt"
	"github.com/archa347/ps-network-test/config"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"time"
)

type Reporter struct {
	dyno       string
	redis      *redis.Client
	intervalMs int
}

func NewReporter(cfg config.Config, client *redis.Client) *Reporter {
	return &Reporter{
		dyno:       cfg.DynoID,
		redis:      client,
		intervalMs: cfg.LivenessInterval,
	}
}

func (l *Reporter) Start() {
	ch := make(chan byte)

	go l.consumer(ch)
	go l.producer(ch)
}

func (l *Reporter) livenessKey() string {
	return fmt.Sprintf("liveness:%v", l.dyno)
}

func (l *Reporter) consumer(ch chan byte) {
	ctx := context.Background()
	logger := log.WithField("at", "Reporter.consumer").WithField("dyno", l.dyno)
	for _ = range ch {
		logger.Infof("Reporting liveness")
		l.redis.Set(ctx, l.livenessKey(), "healthy", 1*time.Minute)
		l.redis.SAdd(ctx, "dynos", l.dyno)
	}
}

func (l *Reporter) producer(ch chan byte) {
	for {
		time.Sleep(time.Duration(l.intervalMs) * time.Millisecond)
		ch <- 0
	}
}
