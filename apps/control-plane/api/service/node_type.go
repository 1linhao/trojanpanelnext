package service

import (
	"trojan-panel/dao"
	"trojan-panel/model/vo"
)

func SelectNodeTypeList() ([]vo.NodeTypeVo, error) {
	return dao.SelectNodeTypeList()
}
