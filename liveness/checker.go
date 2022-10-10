package liveness

import (
	"context"
	"fmt"
	"github.com/archa347/ps-network-test/config"
	"github.com/carlmjohnson/requests"
	"github.com/go-redis/redis/v8"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Checker struct {
	appName          string
	dyno             string
	defaultNATCheck  string
	redis            *redis.Client
	checkPrivateCron string
	checkNATCron     string
	checkDMZCron     string
}

func NewChecker(cfg config.Config, red *redis.Client) *Checker {
	return &Checker{
		appName:          cfg.AppName,
		dyno:             cfg.DynoID,
		defaultNATCheck:  "https://www.google.com",
		redis:            red,
		checkPrivateCron: cfg.PrivateCheckCron,
		checkDMZCron:     cfg.DMZCheckCron,
		checkNATCron:     cfg.NATCheckCron,
	}
}

func (c *Checker) CheckPrivate(ctx context.Context) {
	dynos := c.getDynos(ctx)
	for _, dyno := range dynos {
		c.CheckURL(ctx, fmt.Sprintf("http://%v/private", dyno), "private")
	}
}

func (c *Checker) CheckNAT(ctx context.Context) {
	url, err := c.getExternalURL(ctx)
	if err != nil || url == "" {
		return
	}
	c.CheckURL(ctx, url, "nat")
}

func (c *Checker) CheckDMZ(ctx context.Context) {
	c.CheckURL(ctx, c.getDMZURL(), "dmz")
}

func (c *Checker) checkKey(resultType string, url string) string {
	return fmt.Sprintf("%v:src:%v:dest:%v", resultType, c.dyno, url)
}

func (c *Checker) CheckURL(ctx context.Context, url string, resultType string) error {
	logger := log.WithFields(log.Fields{
		"fn":   "Reporter.CheckURL",
		"url":  url,
		"type": resultType,
	})

	logger.Info()

	err := requests.URL(url).Method(http.MethodGet).Fetch(ctx)
	var result string
	if err != nil {
		result = fmt.Sprintf("fail:%v:%v", err, timeTag())
	} else {
		result = fmt.Sprintf("pass:%v", timeTag())
	}
	logger.WithField("result", result).Info()

	_, err = c.redis.Set(ctx, c.checkKey(resultType, url), result, 0).Result()
	if err != nil {
		logger.WithError(err).Error("Unable to set check result")
		return err
	}
	return nil
}

func (c *Checker) getDynos(ctx context.Context) []string {
	dynos, err := c.redis.SMembers(ctx, "dynos").Result()
	if err != nil {
		log.WithError(err).Warn("Unable to fetch dynos from redis")
	}
	return dynos
}

func (c *Checker) getDMZURL() string {
	return fmt.Sprintf("https://%v.herokuapp.com/dmz", c.appName)
}

func (c *Checker) getExternalURL(ctx context.Context) (string, error) {
	url, err := c.redis.Get(ctx, "settings:NATCheckURL").Result()
	if err != nil {
		log.WithError(err).Error("Unable to fetch NATCheckURL from Redis")
		return "", err
	}
	if url != "" {
		log.Info("No NATCheckURL set")
		return url, nil
	}
	return c.defaultNATCheck, nil
}

func (c *Checker) Start() {
	crn := cron.New(cron.WithSeconds())
	_, err := crn.AddFunc(c.checkPrivateCron, func() { c.CheckPrivate(context.Background()) })
	_, err = crn.AddFunc(c.checkNATCron, func() { c.CheckNAT(context.Background()) })
	_, err = crn.AddFunc(c.checkDMZCron, func() { c.CheckDMZ(context.Background()) })
	if err != nil {
		log.WithError(err).Error("Unable to start checker crons")
	}

	crn.Start()
}
