package dao

import (
	"errors"
	"fmt"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

func SelectNodeServer(where map[string]interface{}) (*model.NodeServer, error) {
	var nodeServer model.NodeServer
	selectFields := []string{"id", "ip", "grpc_port", "grpc_tls_mode", "grpc_tls_server_name", "traffic_period", "traffic_limit_mode", "traffic_total_limit", "traffic_upload_limit", "traffic_download_limit", "`name`", "create_time"}
	buildSelect, values, err := builder.BuildSelect("node_server", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	if err = scanner.Scan(rows, &nodeServer); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.NodeNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &nodeServer, nil
}

func CreateNodeServer(nodeServer *model.NodeServer) error {
	nodeServerEntity := map[string]interface{}{
		"ip": *nodeServer.Ip, "name": *nodeServer.Name,
		"grpc_tls_mode": "mtls", "grpc_tls_server_name": *nodeServer.GrpcTLSServerName,
		"traffic_period": *nodeServer.TrafficPeriod, "traffic_limit_mode": *nodeServer.TrafficLimitMode,
		"traffic_total_limit": *nodeServer.TrafficTotalLimit, "traffic_upload_limit": *nodeServer.TrafficUploadLimit,
		"traffic_download_limit": *nodeServer.TrafficDownloadLimit,
	}
	if nodeServer.GrpcPort != nil && *nodeServer.GrpcPort != 0 {
		nodeServerEntity["grpc_port"] = *nodeServer.GrpcPort
	}

	var data []map[string]interface{}
	data = append(data, nodeServerEntity)
	buildInsert, values, err := builder.BuildInsert("node_server", data)
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

func SelectNodeServerPage(queryName *string, queryIp *string, pageNum *uint, pageSize *uint) (*[]model.NodeServer, uint, error) {
	var (
		total       uint
		nodeServers []model.NodeServer
	)

	// 查询总数
	var whereCount = map[string]interface{}{}
	if queryName != nil && *queryName != "" {
		whereCount["name like"] = fmt.Sprintf("%%%s%%", *queryName)
	}
	if queryIp != nil && *queryIp != "" {
		whereCount["ip like"] = fmt.Sprintf("%%%s%%", *queryIp)
	}
	selectFieldsCount := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("node_server", whereCount, selectFieldsCount)
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
		"_orderby": "create_time desc",
		"_limit":   []uint{(*pageNum - 1) * *pageSize, *pageSize}}
	if queryName != nil && *queryName != "" {
		where["name like"] = fmt.Sprintf("%%%s%%", *queryName)
	}
	if queryIp != nil && *queryIp != "" {
		where["ip like"] = fmt.Sprintf("%%%s%%", *queryIp)
	}
	selectFields := []string{"id", "`ip`", "grpc_port", "grpc_tls_mode", "grpc_tls_server_name", "traffic_period", "traffic_limit_mode", "traffic_total_limit", "traffic_upload_limit", "traffic_download_limit", "name", "create_time"}
	selectSQL, values, err := builder.BuildSelect("node_server", where, selectFields)
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

	if err = scanner.Scan(rows, &nodeServers); err != nil {
		logrus.Errorln(err.Error())
		return nil, 0, errors.New(constant.SysError)
	}
	return &nodeServers, total, nil
}

func DeleteNodeServerById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("node_server", map[string]interface{}{"id": *id})
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

func UpdateNodeServerById(nodeServer *model.NodeServer) error {
	where := map[string]interface{}{"id": *nodeServer.Id}
	update := map[string]interface{}{}
	if nodeServer.Name != nil {
		update["name"] = *nodeServer.Name
	}
	if nodeServer.Ip != nil {
		update["ip"] = *nodeServer.Ip
	}
	if nodeServer.GrpcPort != nil && *nodeServer.GrpcPort != 0 {
		update["grpc_port"] = *nodeServer.GrpcPort
	}
	if nodeServer.GrpcTLSServerName != nil {
		update["grpc_tls_server_name"] = *nodeServer.GrpcTLSServerName
	}
	if nodeServer.TrafficPeriod != nil {
		update["traffic_period"] = *nodeServer.TrafficPeriod
	}
	if nodeServer.TrafficLimitMode != nil {
		update["traffic_limit_mode"] = *nodeServer.TrafficLimitMode
	}
	if nodeServer.TrafficTotalLimit != nil {
		update["traffic_total_limit"] = *nodeServer.TrafficTotalLimit
	}
	if nodeServer.TrafficUploadLimit != nil {
		update["traffic_upload_limit"] = *nodeServer.TrafficUploadLimit
	}
	if nodeServer.TrafficDownloadLimit != nil {
		update["traffic_download_limit"] = *nodeServer.TrafficDownloadLimit
	}
	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("node_server", where, update)
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

func CountNodeServer() (int, error) {
	return CountNodeServerByName(nil, nil)
}

func CountNodeServerByName(id *uint, queryName *string) (int, error) {
	var count int

	var whereCount = map[string]interface{}{}
	if id != nil {
		whereCount["id <>"] = *id
	}
	if queryName != nil {
		whereCount["name"] = *queryName
	}

	selectFields := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("node_server", whereCount, selectFields)
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

func SelectNodeServerList(ip *string, name *string) ([]model.NodeServer, error) {
	var nodeServers []model.NodeServer
	where := map[string]interface{}{
		"_orderby": "create_time desc"}
	if ip != nil && *ip != "" {
		where["ip like"] = fmt.Sprintf("%%%s%%", *ip)
	}
	if name != nil && *name != "" {
		where["name like"] = fmt.Sprintf("%%%s%%", *name)
	}
	selectFields := []string{"id", "`ip`", "name", "grpc_tls_mode", "grpc_tls_server_name", "traffic_period", "traffic_limit_mode", "traffic_total_limit", "traffic_upload_limit", "traffic_download_limit", "create_time"}
	selectSQL, values, err := builder.BuildSelect("node_server", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(selectSQL, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &nodeServers); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return nodeServers, nil
}

// SelectNodeServersForControl returns each server once with the transport
// settings required for panel-to-core control calls.
func SelectNodeServersForControl() ([]model.NodeServer, error) {
	var nodeServers []model.NodeServer
	query := `SELECT DISTINCT ns.id, ns.ip, ns.grpc_port, ns.grpc_tls_mode, ns.grpc_tls_server_name
		FROM node_server ns INNER JOIN node n ON n.node_server_id = ns.id`
	rows, err := db.Query(query)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()
	if err = scanner.Scan(rows, &nodeServers); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return nodeServers, nil
}

func SelectNodeServerAll() ([]vo.NodeServerExportVo, error) {
	var nodeServerExportVo []vo.NodeServerExportVo
	selectFields := []string{"ip", "name", "grpc_port", "grpc_tls_mode", "grpc_tls_server_name", "traffic_period", "traffic_limit_mode", "traffic_total_limit", "traffic_upload_limit", "traffic_download_limit", "create_time"}
	selectSQL, values, err := builder.BuildSelect("node_server", nil, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nodeServerExportVo, errors.New(constant.SysError)
	}

	rows, err := db.Query(selectSQL, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nodeServerExportVo, errors.New(constant.SysError)
	}
	defer rows.Close()
	if err = scanner.Scan(rows, &nodeServerExportVo); err != nil {
		logrus.Errorln(err.Error())
		return nodeServerExportVo, errors.New(constant.SysError)
	}
	return nodeServerExportVo, nil
}

func EnableNodeServerMTLS(id uint, serverName string) error {
	result, err := db.Exec("UPDATE node_server SET grpc_tls_mode='mtls', grpc_tls_server_name=? WHERE id=?", serverName, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New(constant.NodeNotExist)
	}
	return nil
}

// CreateOrUpdateNodeServer 插入数据时，如果数据已经存在，则更新数据；如果数据不存在，则插入新数据
func CreateOrUpdateNodeServer(nodeServerModule model.NodeServer, cover uint) error {
	if nodeServerModule.GrpcTLSServerName == nil || *nodeServerModule.GrpcTLSServerName == "" {
		return errors.New("imported node servers must include grpcTlsServerName for mTLS")
	}
	nodeServer, err := SelectNodeServer(map[string]interface{}{"ip": *nodeServerModule.Ip})
	if err != nil && err.Error() != constant.NodeNotExist {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if nodeServer != nil && cover == 1 {
		// 如果存在则更新，不存在则忽略
		accountWhere := map[string]interface{}{
			"name": *nodeServerModule.Name,
		}

		accountUpdate := map[string]interface{}{}
		if nodeServerModule.Ip != nil && *nodeServerModule.Ip != "" {
			accountUpdate["ip"] = *nodeServerModule.Ip
		}
		if nodeServerModule.GrpcPort != nil && *nodeServerModule.GrpcPort != 0 {
			accountUpdate["grpc_port"] = *nodeServerModule.GrpcPort
		}
		accountUpdate["grpc_tls_server_name"] = *nodeServerModule.GrpcTLSServerName
		if len(accountUpdate) > 0 {
			buildInsert, values, err := builder.BuildUpdate("node_server", accountWhere, accountUpdate)
			if err != nil {
				logrus.Errorln(err.Error())
				return errors.New(constant.SysError)
			}
			if _, err = db.Exec(buildInsert, values...); err != nil {
				logrus.Errorln(err.Error())
				return errors.New(constant.SysError)
			}
		}
	} else {
		if nodeServer == nil {
			// 如果存在则忽略，不存在则添加
			if nodeServer == nil {
				var data []map[string]interface{}
				accountCreate := map[string]interface{}{}
				if nodeServerModule.Name != nil && *nodeServerModule.Name != "" {
					accountCreate["name"] = *nodeServerModule.Name
				}
				if nodeServerModule.Ip != nil && *nodeServerModule.Ip != "" {
					accountCreate["ip"] = *nodeServerModule.Ip
				}
				if nodeServerModule.GrpcPort != nil && *nodeServerModule.GrpcPort != 0 {
					accountCreate["grpc_port"] = *nodeServerModule.GrpcPort
				}
				accountCreate["grpc_tls_mode"] = "mtls"
				accountCreate["grpc_tls_server_name"] = *nodeServerModule.GrpcTLSServerName
				if len(accountCreate) > 0 {
					data = append(data, accountCreate)
					buildInsert, values, err := builder.BuildInsert("node_server", data)
					if err != nil {
						logrus.Errorln(err.Error())
						return errors.New(constant.SysError)
					}
					if _, err = db.Exec(buildInsert, values...); err != nil {
						logrus.Errorln(err.Error())
						return errors.New(constant.SysError)
					}
				}
			}
		}
	}
	return nil
}
