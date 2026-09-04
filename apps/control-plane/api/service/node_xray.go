package service

import (
	"trojan-panel/dao"
	"trojan-panel/model"
)

func SelectNodeXrayById(id *uint) (*model.NodeXray, error) {
	return dao.SelectNodeXrayById(id)
}
