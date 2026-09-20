package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
	"trojan-panel/api"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/middleware"
	"trojan-panel/nodeidentity"
	"trojan-panel/router"
)

func main() {
	if core.NodeIdentityCommandRequested() {
		os.Exit(nodeidentity.Run(os.Args, os.Stdout, os.Stderr))
	}
	core.InitConfig()
	if core.VerifySysadminCredentialRequested() {
		verifySysadminCredential()
		return
	}

	middleware.InitLog()
	dao.InitMySQL()
	redis.InitRedis()
	middleware.InitCron()
	middleware.InitRateLimiter()
	api.InitValidator()

	serverConfig := core.Config.ServerConfig
	r := gin.Default()
	router.Router(r)
	_ = r.Run(fmt.Sprintf(":%d", serverConfig.Port))
	defer releaseResource()
}

func verifySysadminCredential() {
	if err := dao.InitMySQLReadOnly(); err != nil {
		os.Exit(1)
	}
	defer dao.CloseDb()
	if err := dao.VerifyInitialSysadminCredential(); err != nil {
		if errors.Is(err, dao.ErrSysadminCredentialUnhealthy) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func releaseResource() {
	dao.CloseDb()
	redis.CloseRedis()
}
