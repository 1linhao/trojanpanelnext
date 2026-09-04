package dao

import (
	"errors"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

func SelectNodeHysteriaById(id *uint) (*model.NodeHysteria, error) {
	var nodeHysteria model.NodeHysteria
	where := map[string]interface{}{"id": *id}
	selectFields := []string{"id", "protocol", "obfs", "up_mbps", "down_mbps", "server_name", "insecure", "fast_open"}
	buildSelect, values, err := builder.BuildSelect("node_hysteria", where, selectFields)
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

	if err = scanner.Scan(rows, &nodeHysteria); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.NodeNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &nodeHysteria, nil
}

func CreateNodeHysteria(nodeHysteria *model.NodeHysteria) (uint, error) {
	nodeHysteriaCreate := map[string]interface{}{}
	if nodeHysteria.Protocol != nil && *nodeHysteria.Protocol != "" {
		nodeHysteriaCreate["protocol"] = *nodeHysteria.Protocol
	}
	if nodeHysteria.Obfs != nil && *nodeHysteria.Obfs != "" {
		nodeHysteriaCreate["obfs"] = *nodeHysteria.Obfs
	}
	if nodeHysteria.UpMbps != nil {
		nodeHysteriaCreate["up_mbps"] = *nodeHysteria.UpMbps
	}
	if nodeHysteria.DownMbps != nil {
		nodeHysteriaCreate["down_mbps"] = *nodeHysteria.DownMbps
	}
	if nodeHysteria.ServerName != nil && *nodeHysteria.ServerName != "" {
		nodeHysteriaCreate["server_name"] = *nodeHysteria.ServerName
	}
	if nodeHysteria.Insecure != nil {
		nodeHysteriaCreate["insecure"] = *nodeHysteria.Insecure
	}
	if nodeHysteria.FastOpen != nil {
		nodeHysteriaCreate["fast_open"] = *nodeHysteria.FastOpen
	}
	if len(nodeHysteriaCreate) > 0 {
		var data []map[string]interface{}
		data = append(data, nodeHysteriaCreate)
		buildInsert, values, err := builder.BuildInsert("node_hysteria", data)
		if err != nil {
			logrus.Errorln(err.Error())
			return 0, errors.New(constant.SysError)
		}
		result, err := db.Exec(buildInsert, values...)
		if err != nil {
			logrus.Errorln(err.Error())
			return 0, errors.New(constant.SysError)
		}
		id, err := result.LastInsertId()
		if err != nil {
			logrus.Errorln(err.Error())
			return 0, errors.New(constant.SysError)
		}
		return uint(id), nil
	}
	return 0, errors.New(constant.SysError)
}

func UpdateNodeHysteriaById(nodeHysteria *model.NodeHysteria) error {
	where := map[string]interface{}{"id": *nodeHysteria.Id}
	update := map[string]interface{}{}
	if nodeHysteria.Protocol != nil && *nodeHysteria.Protocol != "" {
		update["protocol"] = *nodeHysteria.Protocol
	}
	if nodeHysteria.Obfs != nil && *nodeHysteria.Obfs != "" {
		update["obfs"] = *nodeHysteria.Obfs
	}
	if nodeHysteria.UpMbps != nil {
		update["up_mbps"] = *nodeHysteria.UpMbps
	}
	if nodeHysteria.DownMbps != nil {
		update["down_mbps"] = *nodeHysteria.DownMbps
	}
	if nodeHysteria.ServerName != nil {
		update["server_name"] = *nodeHysteria.ServerName
	}
	if nodeHysteria.Insecure != nil {
		update["insecure"] = *nodeHysteria.Insecure
	}
	if nodeHysteria.FastOpen != nil {
		update["fast_open"] = *nodeHysteria.FastOpen
	}
	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("node_hysteria", where, update)
		if err != nil {
			logrus.Errorln(err.Error())
			return errors.New(constant.SysError)
		}

		if _, err = db.Exec(buildUpdate, values...); err != nil {
			logrus.Errorln(err.Error())
			return errors.New(constant.SysError)
		}
	}
	return nil
}

func DeleteNodeHysteriaById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("node_hysteria", map[string]interface{}{"id": *id})
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err = db.Exec(buildDelete, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}
