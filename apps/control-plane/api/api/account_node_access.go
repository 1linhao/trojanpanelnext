package api

import (
	"errors"
	"net/http"
	"trojan-panel/dao"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func SelectAccountNodeAccess(c *gin.Context) {
	if !requireCurrentSysAdmin(c) {
		return
	}
	var query struct {
		SyncOnly bool `form:"syncOnly"`
		ID       uint `form:"id" binding:"required"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	status, err := dao.SelectAccountNodeSync(query.ID)
	if err != nil {
		failAccountNodeAccess(c, err)
		return
	}
	if query.SyncOnly {
		vo.Success(status, c)
		return
	}
	access, err := dao.SelectAccountNodeAccess(query.ID)
	if err != nil {
		failAccountNodeAccess(c, err)
		return
	}
	options, err := dao.SelectAccountNodeOptions()
	if err != nil {
		failAccountNodeAccess(c, err)
		return
	}
	vo.Success(struct {
		dao.AccountNodeAccess
		Nodes []dao.AccountNodeOption `json:"nodes"`
		Sync  dao.AccountNodeSync     `json:"sync"`
	}{access, options, status}, c)
}

func UpdateAccountNodeAccess(c *gin.Context) {
	if !requireCurrentSysAdmin(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	var request struct {
		ID         uint   `json:"id" binding:"required"`
		Restricted *bool  `json:"restricted" binding:"required"`
		NodeIDs    []uint `json:"nodeIds" binding:"required,dive,gt=0"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || (!*request.Restricted && len(request.NodeIDs) != 0) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	revision, err := service.UpdateAccountNodeAccess(request.ID, dao.AccountNodeAccess{
		Restricted: *request.Restricted,
		NodeIDs:    request.NodeIDs,
	})
	if err != nil {
		failAccountNodeAccess(c, err)
		return
	}
	vo.Success(struct {
		Revision uint64 `json:"revision,string"`
	}{revision}, c)
}

func requireCurrentSysAdmin(c *gin.Context) bool {
	allowed, err := service.IsCurrentSysAdmin(accountManagementToken(c))
	if err != nil {
		vo.Fail(err.Error(), c)
		return false
	}
	if !allowed {
		vo.Fail(constant.ForbiddenError, c)
	}
	return allowed
}

func failAccountNodeAccess(c *gin.Context, err error) {
	switch {
	case errors.Is(err, dao.ErrNodeAccessAgentsNotReady):
		vo.Fail(err.Error(), c)
	case errors.Is(err, dao.ErrNodeAccessInvalidNode):
		vo.Fail(constant.NodeNotExist, c)
	case err.Error() == constant.UnauthorizedError:
		vo.Fail(constant.UnauthorizedError, c)
	default:
		logrus.WithError(err).Error("account node access")
		vo.Fail(constant.SysError, c)
	}
}
