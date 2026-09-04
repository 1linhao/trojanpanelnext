package dao

import (
	"errors"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

func SelectNodeHysteria2ById(id *uint) (*model.NodeHysteria2, error) {
	var nodeHysteria2 model.NodeHysteria2
	where := map[string]interface{}{"id": *id}
	selectFields := []string{"id", "obfs_password", "up_mbps", "down_mbps", "server_name", "insecure", "port_hopping", "hop_interval"}
	buildSelect, values, err := builder.BuildSelect("node_hysteria2", where, selectFields)
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

	if err = scanner.Scan(rows, &nodeHysteria2); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.NodeNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &nodeHysteria2, nil
}

func CreateNodeHysteria2(nodeHysteria2 *model.NodeHysteria2) (uint, error) {
	nodeHysteria2Create := map[string]interface{}{}
	if nodeHysteria2.ObfsPassword != nil {
		nodeHysteria2Create["obfs_password"] = *nodeHysteria2.ObfsPassword
	}
	if nodeHysteria2.UpMbps != nil {
		nodeHysteria2Create["up_mbps"] = *nodeHysteria2.UpMbps
	}
	if nodeHysteria2.DownMbps != nil {
		nodeHysteria2Create["down_mbps"] = *nodeHysteria2.DownMbps
	}
	if nodeHysteria2.ServerName != nil {
		nodeHysteria2Create["server_name"] = *nodeHysteria2.ServerName
	}
	if nodeHysteria2.Insecure != nil {
		nodeHysteria2Create["insecure"] = *nodeHysteria2.Insecure
	}
	if nodeHysteria2.PortHopping != nil {
		nodeHysteria2Create["port_hopping"] = *nodeHysteria2.PortHopping
	}
	if nodeHysteria2.HopInterval != nil {
		nodeHysteria2Create["hop_interval"] = *nodeHysteria2.HopInterval
	}
	if len(nodeHysteria2Create) > 0 {
		var data []map[string]interface{}
		data = append(data, nodeHysteria2Create)
		buildInsert, values, err := builder.BuildInsert("node_hysteria2", data)
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

func UpdateNodeHysteria2ById(nodeHysteria2 *model.NodeHysteria2) error {
	where := map[string]interface{}{"id": *nodeHysteria2.Id}
	update := map[string]interface{}{}
	if nodeHysteria2.ObfsPassword != nil {
		update["obfs_password"] = *nodeHysteria2.ObfsPassword
	}
	if nodeHysteria2.UpMbps != nil {
		update["up_mbps"] = *nodeHysteria2.UpMbps
	}
	if nodeHysteria2.DownMbps != nil {
		update["down_mbps"] = *nodeHysteria2.DownMbps
	}
	if nodeHysteria2.ServerName != nil {
		update["server_name"] = *nodeHysteria2.ServerName
	}
	if nodeHysteria2.Insecure != nil {
		update["insecure"] = *nodeHysteria2.Insecure
	}
	if nodeHysteria2.PortHopping != nil {
		update["port_hopping"] = *nodeHysteria2.PortHopping
	}
	if nodeHysteria2.HopInterval != nil {
		update["hop_interval"] = *nodeHysteria2.HopInterval
	}
	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("node_hysteria2", where, update)
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

func DeleteNodeHysteria2ById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("node_hysteria2", map[string]interface{}{"id": *id})
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
