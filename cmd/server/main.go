package main

import (
	"github.com/archa347/ps-network-test/config"
	"github.com/archa347/ps-network-test/liveness"
	"github.com/archa347/ps-network-test/redis"
	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	"os"
)

func main() {
	cfg := config.New()

	redisClient := redis.RedisClient(cfg)

	livenessReporter := liveness.NewReporter(cfg, redisClient)
	livenessChecker := liveness.NewChecker(cfg, redisClient)

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.String(200, "Hello from %v", cfg.DynoID)
	})

	router.GET("/private", func(c *gin.Context) {
		livenessReporter.ReportPrivate(c)
	})

	router.GET("/dmz", func(c *gin.Context) {
		livenessReporter.ReportDMZ(c)
	})

	livenessReporter.Start()
	livenessChecker.Start()
	go router.Run(":" + cfg.Port)
	privateIP := os.Getenv("HEROKU_PRIVATE_IP")
	if privateIP != "" {
		go router.Run(privateIP + ":7777")
	}
}
