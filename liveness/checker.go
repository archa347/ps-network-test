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
	"strings"
	"time"
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

type Report struct {
	Dyno    string
	DMZ     []CheckReport
	NAT     []CheckReport
	Private []CheckReport
}

type CheckReport struct {
	Dest   string
	Result string
}

func (r *CheckReport) Passed() bool {
	return strings.HasPrefix(r.Result, "pass") || strings.HasPrefix(r.Result, "healthy")
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

func (c *Checker) Report(ctx context.Context) map[string]interface{} {
	dynos := c.getDynos(ctx)

	var reports []Report
	for _, dyno := range dynos {
		reports = append(reports, c.dynoReport(ctx, dyno))
	}

	return map[string]interface{}{
		"dynos": reports,
	}
}

func (c *Checker) dynoReport(ctx context.Context, dynoID string) Report {
	return Report{
		Dyno:    dynoID,
		DMZ:     c.dynoDMZReport(ctx, dynoID),
		NAT:     c.dynoCheckReports(ctx, dynoID, "nat"),
		Private: c.dynoCheckReports(ctx, dynoID, "private"),
	}
}

func (c *Checker) dynoDMZReport(ctx context.Context, dynoID string) []CheckReport {
	result, err := c.redis.Get(ctx, "dmz:"+dynoID).Result()
	if err != nil {
		log.WithError(err).Error("Unable to get dmz check report")
		return []CheckReport{}
	}
	return []CheckReport{{
		Result: result,
	}}
}

func (c *Checker) dynoCheckReports(ctx context.Context, dynoID string, checkType string) []CheckReport {
	var reports []CheckReport
	var cursor uint64 = 0
	for {
		var keys []string
		var err error
		keys, cursor, err = c.redis.Scan(ctx, cursor, c.checkKeyPrefix(checkType, dynoID)+":dest:*", 10).Result()
		if err != nil {
			log.WithError(err).Warn("Unable to fetch results from redis")
			break
		}
		values, err := c.redis.MGet(ctx, keys...).Result()
		if err != nil {
			log.WithError(err).Warn("Unable to fetch results from redis")
			break
		}
		for i, value := range values {
			reports = append(reports, CheckReport{
				Dest:   strings.TrimPrefix(keys[i], c.checkKeyPrefix(checkType, dynoID)+":dest:"),
				Result: fmt.Sprintf("%v", value),
			})
		}

		if cursor == 0 {
			break
		}
	}
	return reports
}

func (c *Checker) CheckPrivate(ctx context.Context) {
	dynos := c.getDynos(ctx)
	for _, dyno := range dynos {
		c.CheckURL(ctx, fmt.Sprintf("http://%v:7777/private", dyno), "private")
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
	return fmt.Sprintf("%v:dest:%v", c.checkKeyPrefix(resultType, c.dyno), url)
}

func (c *Checker) checkKeyPrefix(resultType string, dynoId string) string {
	return fmt.Sprintf("%v:src:%v", resultType, dynoId)
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

	_, err = c.redis.Set(ctx, c.checkKey(resultType, url), result, 10*time.Minute).Result()
	if err != nil {
		logger.WithError(err).Error("Unable to set check result")
		return err
	}
	return nil
}

func (c *Checker) getDynos(ctx context.Context) []string {
	dynos := make([]string, 0)

	var cursor uint64 = 0
	for {
		var keys []string
		var err error
		keys, cursor, err = c.redis.Scan(ctx, cursor, "liveness:*", 10).Result()
		if err != nil {
			log.WithError(err).Warn("Unable to fetch dynos from redis")
			break
		}
		for _, key := range keys {
			dynos = append(dynos, strings.TrimPrefix(key, "liveness:"))
		}

		if cursor == 0 {
			break
		}
	}

	return dynos
}

func (c *Checker) getDMZURL() string {
	return fmt.Sprintf("https://%v.herokuapp.com/dmz", c.appName)
}

func (l *Checker) dmzReportKey() string {
	return fmt.Sprintf("dmz:%v", l.dyno)
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
