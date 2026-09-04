package dao

import (
	"errors"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

func SelectNodeTypeList() ([]vo.NodeTypeVo, error) {
	var nodeTypes []model.NodeType

	buildSelect, values, err := builder.NamedQuery(
		"select id,`name` from node_type where id not in (2,3) order by create_time desc", nil)
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

	if err = scanner.Scan(rows, &nodeTypes); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	var nodeTypeVos []vo.NodeTypeVo
	for _, item := range nodeTypes {
		nodeTypeVos = append(nodeTypeVos, vo.NodeTypeVo{
			Id:   *item.Id,
			Name: *item.Name,
		})
	}
	return nodeTypeVos, nil
}

func SelectNodeTypeById(id *uint) (*model.NodeType, error) {
	var nodeType model.NodeType
	buildSelect, values, err := builder.NamedQuery(
		"select id,`name` from node_type where id = {{id}}", map[string]interface{}{"id": *id})
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

	if err = scanner.Scan(rows, &nodeType); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.NodeTypeNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	return &nodeType, nil
}
