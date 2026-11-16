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
	if !authorizeAccountNodeAccess(c) {
		return
	}
	var query struct {
		ID uint `form:"id" binding:"required"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		vo.Fail(constant.ValidateFailed, c)
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
	c.Header("Cache-Control", "private, no-store")
	vo.Success(struct {
		dao.AccountNodeAccess
		Nodes []dao.AccountNodeOption `json:"nodes"`
	}{access, options}, c)
}

func UpdateAccountNodeAccess(c *gin.Context) {
	if !authorizeAccountNodeAccess(c) {
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
	if err := service.UpdateAccountNodeAccess(request.ID, dao.AccountNodeAccess{
		Restricted: *request.Restricted,
		NodeIDs:    request.NodeIDs,
	}); err != nil {
		failAccountNodeAccess(c, err)
		return
	}
	vo.Success(nil, c)
}

func authorizeAccountNodeAccess(c *gin.Context) bool {
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
