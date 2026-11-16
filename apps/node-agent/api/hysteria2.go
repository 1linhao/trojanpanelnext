package api

import (
	"net"
	"strconv"

	"github.com/gin-gonic/gin"
	"trojan-panel-core/dao"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/dto"
	"trojan-panel-core/model/vo"
)

func Hysteria2Api(c *gin.Context) {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	port, portErr := strconv.ParseUint(c.Query("port"), 10, 16)
	if err != nil || !net.ParseIP(host).IsLoopback() || portErr != nil || port == 0 {
		vo.Hysteria2ApiFail("", c)
		return
	}
	var hysteria2AuthDto dto.Hysteria2AuthDto
	if err := c.ShouldBindJSON(&hysteria2AuthDto); err != nil {
		vo.Hysteria2ApiFail(constant.ValidateFailed, c)
		return
	}
	if err := validate.Struct(&hysteria2AuthDto); err != nil {
		vo.Hysteria2ApiFail(constant.ValidateFailed, c)
		return
	}
	if err := dao.AuthorizeHysteria2Account(*hysteria2AuthDto.Auth, uint(port)); err != nil {
		vo.Hysteria2ApiFail("", c)
		return
	}
	vo.Hysteria2ApiSuccess(*hysteria2AuthDto.Auth, c)
}
