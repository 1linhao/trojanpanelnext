package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
	"strconv"
	"time"
	"trojan-panel-core/api"
	"trojan-panel-core/app"
	"trojan-panel-core/core"
	"trojan-panel-core/dao"
	"trojan-panel-core/dao/redis"
	"trojan-panel-core/healthcheck"
	"trojan-panel-core/middleware"
	"trojan-panel-core/router"
)

func main() {
	core.InitConfig()
	if mode := os.Getenv("TP_VERIFY_NODE_DATA_SERVICES"); mode != "" {
		if err := healthcheck.Verify(mode); err != nil {
			fmt.Fprintln(os.Stderr, "Node data-service health check failed")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "Node data-service health check passed: %s\n", mode)
		return
	}
	if err := healthcheck.Verify("all"); err != nil {
		panic("Node data-service startup health check failed")
	}
	initRuntime()
	startCredentialWatchdog()
	serverConfig := core.Config.ServerConfig
	r := gin.Default()
	router.Router(r)
	_ = r.Run(fmt.Sprintf(":%d", serverConfig.Port))
	defer closeResource()
}

func startCredentialWatchdog() {
	seconds := 30
	if configured := os.Getenv("TP_NODE_CREDENTIAL_RECHECK_SECONDS"); configured != "" {
		value, err := strconv.Atoi(configured)
		if err != nil || value < 1 || value > 3600 {
			panic("TP_NODE_CREDENTIAL_RECHECK_SECONDS must be between 1 and 3600")
		}
		seconds = value
	}
	go func() {
		if err := healthcheck.Watch(context.Background(), time.Duration(seconds)*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, "Node data-service identity is no longer valid; stopping")
			os.Exit(1)
		}
	}()
}

func initRuntime() {
	middleware.InitLog()
	dao.InitMySQL()
	dao.InitSqlLite()
	redis.InitRedis()
	middleware.InitRateLimiter()
	api.InitValidator()
	api.InitGrpcServer()
	app.InitApp()
	middleware.InitCron()
}
func closeResource() {
	dao.CloseDb()
	dao.CloseSqliteDb()
	redis.CloseRedis()
}
