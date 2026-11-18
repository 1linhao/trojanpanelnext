package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"trojan-panel/dao"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

func SelectServerTrafficDisplay(c *gin.Context) {
	if !authorizeServerTrafficDisplay(c) {
		return
	}
	config, err := dao.SelectServerTrafficDisplay()
	if err != nil {
		failServerTrafficDisplay(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	vo.Success(config, c)
}

func UpdateServerTrafficDisplay(c *gin.Context) {
	if !authorizeServerTrafficDisplay(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	var request struct {
		Enabled   *bool  `json:"enabled" binding:"required"`
		ServerIDs []uint `json:"serverIds" binding:"required,dive,gt=0"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := dao.UpdateServerTrafficDisplay(*request.Enabled, request.ServerIDs); err != nil {
		failServerTrafficDisplay(c, err)
		return
	}
	vo.Success(nil, c)
}

func SelectServerTrafficSummaries(c *gin.Context) {
	summaries, err := service.ServerTrafficSummaries()
	if err != nil {
		failServerTrafficDisplay(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	vo.Success(summaries, c)
}

func authorizeServerTrafficDisplay(c *gin.Context) bool {
	claims := service.GetCurrentAccount(c)
	if claims == nil {
		return false
	}
	if !util.IsAdmin(claims.Roles) {
		vo.Fail(constant.ForbiddenError, c)
		return false
	}
	account, err := dao.SelectAccountById(&claims.Id)
	if err != nil {
		failServerTrafficDisplay(c, err)
		return false
	}
	if account.RoleId == nil || (*account.RoleId != constant.SYSADMIN && *account.RoleId != constant.ADMIN) ||
		account.Deleted == nil || *account.Deleted != 0 {
		vo.Fail(constant.ForbiddenError, c)
		return false
	}
	return true
}

func failServerTrafficDisplay(c *gin.Context, err error) {
	if errors.Is(err, dao.ErrServerTrafficDisplayInvalidServer) {
		vo.Fail(err.Error(), c)
		return
	}
	logrus.WithError(err).Error("server traffic display")
	vo.Fail(constant.SysError, c)
}
