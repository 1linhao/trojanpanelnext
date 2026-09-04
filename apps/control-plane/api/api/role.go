package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func SelectRoleList(c *gin.Context) {
	var roleDto dto.RoleDto
	_ = c.ShouldBind(&roleDto)
	if err := validate.Struct(&roleDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	roles, err := service.SelectRoleList(roleDto)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	var roleListVos []vo.RoleListVo
	for _, item := range roles {
		roleListVos = append(roleListVos, vo.RoleListVo{
			Id:   *item.Id,
			Name: *item.Name,
			Desc: *item.Desc,
		})
	}
	vo.Success(roleListVos, c)
}
