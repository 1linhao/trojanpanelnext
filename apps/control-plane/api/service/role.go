package service

import (
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/dto"
)

func SelectRoleList(roleDto dto.RoleDto) ([]model.Role, error) {
	return dao.SelectRoleList(roleDto)
}
