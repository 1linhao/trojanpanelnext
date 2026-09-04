package service

import (
	"trojan-panel/dao"
	"trojan-panel/model"
)

func SelectNodeTrojanGoById(id *uint) (*model.NodeTrojanGo, error) {
	return dao.SelectNodeTrojanGoById(id)
}
