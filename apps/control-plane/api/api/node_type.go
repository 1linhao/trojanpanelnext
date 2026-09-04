package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func SelectNodeTypeList(c *gin.Context) {
	nodeTypeVos, err := service.SelectNodeTypeList()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nodeTypeVos, c)
}
