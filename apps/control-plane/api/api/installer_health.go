package api

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"trojan-panel/dao"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

type installerHealthRequest struct {
	Pass string `json:"pass"`
}

func InstallerHealth(c *gin.Context) {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var request installerHealthRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Pass == "" {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := dao.VerifySysadminPassword(request.Pass); err != nil {
		vo.Fail(constant.UsernameOrPassError, c)
		return
	}
	vo.Success(nil, c)
}
