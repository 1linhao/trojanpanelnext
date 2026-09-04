package dao

import (
	"errors"
	"fmt"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

func SelectRoleList(roleDto dto.RoleDto) ([]model.Role, error) {
	var roles []model.Role

	where := map[string]interface{}{
		"_orderby": "create_time desc",
	}
	if roleDto.Name != nil && *roleDto.Name != "" {
		where["name"] = *roleDto.Name
	}
	if roleDto.Desc != nil && *roleDto.Desc != "" {
		where["desc"] = *roleDto.Desc
	}
	selectFields := []string{"id", "`name`", "`desc`"}
	buildSelect, values, err := builder.BuildSelect("role", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &roles); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	return roles, nil
}

func SelectRoleNameByParentId(id *uint, includeSelf bool) ([]string, error) {
	var roleNames []string
	roleVo, err := SelectRoleById(id)
	if err != nil {
		return nil, err
	}
	if includeSelf {
		if roleVo.Name != "" {
			roleNames = append(roleNames, roleVo.Name)
		}
	}
	buildSelect, values, err := builder.NamedQuery("select `name` from `role` where `path` like concat({{path}},'-','%')",
		map[string]interface{}{"path": fmt.Sprintf("%s%d", roleVo.Path, *id)})
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	result, err := scanner.ScanMap(rows)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	for _, record := range result {
		roleNames = append(roleNames, fmt.Sprintf("%s", record["name"]))
	}
	return roleNames, nil
}

func SelectRoleById(id *uint) (*vo.RoleVo, error) {
	var role model.Role
	buildSelect, values, err := builder.NamedQuery("select id,`name`,`desc`,`path` from `role` where id = {{id}}",
		map[string]interface{}{"id": *id})
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	err = scanner.Scan(rows, &role)
	if err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.RoleNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	roleVo := vo.RoleVo{
		Id:   *role.Id,
		Name: *role.Name,
		Desc: *role.Desc,
		Path: *role.Path,
	}
	return &roleVo, nil
}
