package dao

import (
	"errors"
	"fmt"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

func SelectNodeById(id *uint) (*model.Node, error) {
	var node model.Node
	where := map[string]interface{}{"id": *id}
	selectFields := []string{"id", "node_server_id", "`node_sub_id`", "node_type_id", "name", "node_server_ip", "node_server_grpc_port", "domain", "port", "priority", "client_types", "naive_uot_enable", "naive_uot_version", "create_time"}
	buildSelect, values, err := builder.BuildSelect("node", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	if err = scanner.Scan(rows, &node); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.NodeNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &node, nil
}

func CreateNode(node *model.Node) error {
	nodeEntity := map[string]interface{}{
		"node_server_id": *node.NodeServerId,
		"node_sub_id":    *node.NodeSubId,
		"node_type_id":   *node.NodeTypeId,
		"name":           *node.Name,
		"node_server_ip": *node.NodeServerIp,
		"domain":         *node.Domain,
	}
	if node.NaiveUotEnable != nil {
		nodeEntity["naive_uot_enable"] = *node.NaiveUotEnable
	}
	if node.ClientTypes != nil {
		nodeEntity["client_types"] = *node.ClientTypes
	}
	if node.NaiveUotVersion != nil {
		nodeEntity["naive_uot_version"] = *node.NaiveUotVersion
	}
	if node.Port != nil && *node.Port != 0 {
		nodeEntity["port"] = *node.Port
	}
	if node.Priority != nil {
		nodeEntity["priority"] = *node.Priority
	}
	if node.NodeServerGrpcPort != nil && *node.NodeServerGrpcPort != 0 {
		nodeEntity["node_server_grpc_port"] = *node.NodeServerGrpcPort
	}

	var data []map[string]interface{}
	data = append(data, nodeEntity)
	buildInsert, values, err := builder.BuildInsert("node", data)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	if _, err = db.Exec(buildInsert, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

func SelectNodePage(queryName *string, nodeServerId *uint, pageNum *uint, pageSize *uint) (*[]model.Node, uint, error) {
	var (
		total uint
		nodes []model.Node
	)

	// 查询总数
	var whereCount = map[string]interface{}{
		"node_type_id not in": []uint{constant.TrojanGo, constant.Hysteria},
	}
	if queryName != nil && *queryName != "" {
		whereCount["name like"] = fmt.Sprintf("%%%s%%", *queryName)
	}
	if nodeServerId != nil && *nodeServerId != 0 {
		whereCount["node_server_id"] = *nodeServerId
	}
	selectFieldsCount := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("node", whereCount, selectFieldsCount)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, 0, errors.New(constant.SysError)
	}
	if err = db.QueryRow(buildSelect, values...).Scan(&total); err != nil {
		logrus.Errorln(err.Error())
		return nil, 0, errors.New(constant.SysError)
	}

	// 分页查询
	where := map[string]interface{}{
		"node_type_id not in": []uint{constant.TrojanGo, constant.Hysteria},
		"_orderby":            "priority desc,create_time desc",
		"_limit":              []uint{(*pageNum - 1) * *pageSize, *pageSize}}
	if queryName != nil && *queryName != "" {
		where["name like"] = fmt.Sprintf("%%%s%%", *queryName)
	}
	if nodeServerId != nil && *nodeServerId != 0 {
		where["node_server_id"] = *nodeServerId
	}
	selectFields := []string{"id", "node_server_id", "`node_sub_id`", "node_type_id", "name", "node_server_ip", "node_server_grpc_port", "domain", "port", "priority", "client_types", "naive_uot_enable", "naive_uot_version", "create_time"}
	selectSQL, values, err := builder.BuildSelect("node", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, 0, errors.New(constant.SysError)
	}

	rows, err := db.Query(selectSQL, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, 0, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &nodes); err != nil {
		logrus.Errorln(err.Error())
		return nil, 0, errors.New(constant.SysError)
	}
	return &nodes, total, nil
}

func DeleteNodeById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("node", map[string]interface{}{"id": *id})
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

func UpdateNodeById(node *model.Node) error {
	where := map[string]interface{}{"id": *node.Id}
	update := map[string]interface{}{}
	if node.NodeServerId != nil {
		update["node_server_id"] = *node.NodeServerId
	}
	if node.NodeSubId != nil {
		update["node_sub_id"] = *node.NodeSubId
	}
	if node.NodeTypeId != nil {
		update["node_type_id"] = *node.NodeTypeId
	}
	if node.Name != nil {
		update["name"] = *node.Name
	}
	if node.NodeServerIp != nil {
		update["node_server_ip"] = *node.NodeServerIp
	}
	if node.NodeServerGrpcPort != nil {
		update["node_server_grpc_port"] = *node.NodeServerGrpcPort
	}
	if node.Domain != nil {
		update["domain"] = *node.Domain
	}
	if node.Port != nil {
		update["port"] = *node.Port
	}
	if node.Priority != nil {
		update["priority"] = *node.Priority
	}
	if node.ClientTypes != nil {
		update["client_types"] = *node.ClientTypes
	}
	if node.NaiveUotEnable != nil {
		update["naive_uot_enable"] = *node.NaiveUotEnable
	}
	if node.NaiveUotVersion != nil {
		update["naive_uot_version"] = *node.NaiveUotVersion
	}
	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("node", where, update)
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

func CountNode() (int, error) {
	return CountNodeByNameAndNodeServerId(nil, nil, nil)
}

func CountNodeByIpAndPort(nodeServerIp *string, port *uint) (int, error) {
	var count int

	var whereCount = map[string]interface{}{}
	if nodeServerIp != nil && *nodeServerIp != "" {
		whereCount["node_server_ip"] = *nodeServerIp
	}
	if port != nil && *port != 0 {
		whereCount["port"] = *port
	}

	selectFields := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("node", whereCount, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}

	if err = db.QueryRow(buildSelect, values...).Scan(&count); err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	return count, nil
}

func CountNodeByNameAndNodeServerId(id *uint, queryName *string, nodeServerId *uint) (int, error) {
	var count int

	var whereCount = map[string]interface{}{}
	if id != nil {
		whereCount["id <>"] = *id
	}
	if queryName != nil {
		whereCount["name"] = *queryName
	}
	if nodeServerId != nil {
		whereCount["node_server_id"] = *nodeServerId
	}

	selectFields := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("node", whereCount, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}

	if err = db.QueryRow(buildSelect, values...).Scan(&count); err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	return count, nil
}

func SelectNodes() ([]model.Node, error) {
	var nodes []model.Node

	where := map[string]interface{}{
		"node_type_id not in": []uint{constant.TrojanGo, constant.Hysteria},
		"_orderby":            "priority desc,create_time desc"}
	buildSelect, values, err := builder.BuildSelect("node", where, []string{
		"id", "node_sub_id", "node_type_id", "name", "domain", "port", "client_types", "naive_uot_enable", "naive_uot_version"})
	if err != nil {
		logrus.Errorln(err.Error())
		return nodes, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nodes, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &nodes); err != nil {
		logrus.Errorln(err.Error())
		return nodes, errors.New(constant.SysError)
	}
	return nodes, nil
}

func SelectNodesIpGrpcPortDistinct() ([]model.Node, error) {
	var nodes []model.Node

	buildSelect, values, err := builder.NamedQuery("select node_server_ip, node_server_grpc_port from node group by node_server_ip, node_server_grpc_port", nil)
	if err != nil {
		logrus.Errorln(err.Error())
		return nodes, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nodes, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &nodes); err != nil {
		logrus.Errorln(err.Error())
		return nodes, errors.New(constant.SysError)
	}
	return nodes, nil
}
