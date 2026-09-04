package dao

import (
	"errors"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

func SelectNodeXrayById(id *uint) (*model.NodeXray, error) {
	var nodeXray model.NodeXray
	where := map[string]interface{}{"id": *id}
	selectFields := []string{"id", "`protocol`", "xray_flow", "xray_ss_method", "uot_enable", "uot_version", "xudp_enable", "mux_enable", "reality_pbk", "settings", "stream_settings", "tag", "sniffing", "allocate"}
	buildSelect, values, err := builder.BuildSelect("node_xray", where, selectFields)
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

	if err = scanner.Scan(rows, &nodeXray); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.NodeNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	return &nodeXray, nil
}

func CreateNodeXray(nodeXray *model.NodeXray) (uint, error) {
	nodeXrayCreate := map[string]interface{}{}
	if nodeXray.Protocol != nil && *nodeXray.Protocol != "" {
		nodeXrayCreate["protocol"] = *nodeXray.Protocol
	}
	if nodeXray.XrayFlow != nil && *nodeXray.XrayFlow != "" {
		nodeXrayCreate["xray_flow"] = *nodeXray.XrayFlow
	}
	if nodeXray.XraySSMethod != nil && *nodeXray.XraySSMethod != "" {
		nodeXrayCreate["xray_ss_method"] = *nodeXray.XraySSMethod
	}
	if nodeXray.UotEnable != nil {
		nodeXrayCreate["uot_enable"] = *nodeXray.UotEnable
	}
	if nodeXray.UotVersion != nil {
		nodeXrayCreate["uot_version"] = *nodeXray.UotVersion
	}
	if nodeXray.XudpEnable != nil {
		nodeXrayCreate["xudp_enable"] = *nodeXray.XudpEnable
	}
	if nodeXray.MuxEnable != nil {
		nodeXrayCreate["mux_enable"] = *nodeXray.MuxEnable
	}
	if nodeXray.RealityPbk != nil && *nodeXray.RealityPbk != "" {
		nodeXrayCreate["reality_pbk"] = *nodeXray.RealityPbk
	}
	if nodeXray.Settings != nil && *nodeXray.Settings != "" {
		nodeXrayCreate["settings"] = *nodeXray.Settings
	}
	if nodeXray.StreamSettings != nil && *nodeXray.StreamSettings != "" {
		nodeXrayCreate["stream_settings"] = *nodeXray.StreamSettings
	}
	if nodeXray.Tag != nil && *nodeXray.Tag != "" {
		nodeXrayCreate["tag"] = *nodeXray.Tag
	}
	if nodeXray.Sniffing != nil && *nodeXray.Sniffing != "" {
		nodeXrayCreate["sniffing"] = *nodeXray.Sniffing
	}
	if nodeXray.Allocate != nil && *nodeXray.Allocate != "" {
		nodeXrayCreate["allocate"] = *nodeXray.Allocate
	}
	if len(nodeXrayCreate) > 0 {
		var data []map[string]interface{}
		data = append(data, nodeXrayCreate)
		buildInsert, values, err := builder.BuildInsert("node_xray", data)
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

func UpdateNodeXrayById(nodeXray *model.NodeXray) error {
	where := map[string]interface{}{"id": *nodeXray.Id}
	update := map[string]interface{}{}
	if nodeXray.Protocol != nil && *nodeXray.Protocol != "" {
		update["protocol"] = *nodeXray.Protocol
	}
	if nodeXray.XrayFlow != nil && *nodeXray.XrayFlow != "" {
		update["xray_flow"] = *nodeXray.XrayFlow
	}
	if nodeXray.XraySSMethod != nil && *nodeXray.XraySSMethod != "" {
		update["xray_ss_method"] = *nodeXray.XraySSMethod
	}
	if nodeXray.UotEnable != nil {
		update["uot_enable"] = *nodeXray.UotEnable
	}
	if nodeXray.UotVersion != nil {
		update["uot_version"] = *nodeXray.UotVersion
	}
	if nodeXray.XudpEnable != nil {
		update["xudp_enable"] = *nodeXray.XudpEnable
	}
	if nodeXray.MuxEnable != nil {
		update["mux_enable"] = *nodeXray.MuxEnable
	}
	if nodeXray.RealityPbk != nil && *nodeXray.RealityPbk != "" {
		update["reality_pbk"] = *nodeXray.RealityPbk
	}
	if nodeXray.Settings != nil && *nodeXray.Settings != "" {
		update["settings"] = *nodeXray.Settings
	}
	if nodeXray.StreamSettings != nil && *nodeXray.StreamSettings != "" {
		update["stream_settings"] = *nodeXray.StreamSettings
	}
	if nodeXray.Tag != nil && *nodeXray.Tag != "" {
		update["tag"] = *nodeXray.Tag
	}
	if nodeXray.Sniffing != nil && *nodeXray.Sniffing != "" {
		update["sniffing"] = *nodeXray.Sniffing
	}
	if nodeXray.Allocate != nil && *nodeXray.Allocate != "" {
		update["allocate"] = *nodeXray.Allocate
	}
	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("node_xray", where, update)
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

func DeleteNodeXrayById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("node_xray", map[string]interface{}{"id": *id})
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
