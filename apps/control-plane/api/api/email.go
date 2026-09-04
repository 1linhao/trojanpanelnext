package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func SendEmail(c *gin.Context) {
	var sendEmailDto dto.SendEmailDto
	_ = c.ShouldBindJSON(&sendEmailDto)
	if err := service.SendEmail(&sendEmailDto); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}
