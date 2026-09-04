package service

import (
	"trojan-panel/dao"
	"trojan-panel/model"
)

func SelectNodeHysteriaById(id *uint) (*model.NodeHysteria, error) {
	return dao.SelectNodeHysteriaById(id)
}
