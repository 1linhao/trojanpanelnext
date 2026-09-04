package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/middleware"
	"trojan-panel/router"
)

func main() {
	serverConfig := core.Config.ServerConfig
	r := gin.Default()
	router.Router(r)
	_ = r.Run(fmt.Sprintf(":%d", serverConfig.Port))
	defer releaseResource()
}

func init() {
	core.InitConfig()
	middleware.InitLog()
	dao.InitMySQL()
	redis.InitRedis()
	middleware.InitCron()
	middleware.InitRateLimiter()
	api.InitValidator()
}

func releaseResource() {
	dao.CloseDb()
	redis.CloseRedis()
}
