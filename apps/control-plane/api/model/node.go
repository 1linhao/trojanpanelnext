package model

import "time"

type Node struct {
	Id                 *uint      `ddb:"id"`
	NodeServerId       *uint      `ddb:"node_server_id"`
	NodeSubId          *uint      `ddb:"node_sub_id"`
	NodeTypeId         *uint      `ddb:"node_type_id"`
	Name               *string    `ddb:"name"`
	NodeServerIp       *string    `ddb:"node_server_ip"`
	NodeServerGrpcPort *uint      `ddb:"node_server_grpc_port"`
	Domain             *string    `ddb:"domain"`
	Port               *uint      `ddb:"port"`
	ExternalPort       *uint      `ddb:"external_port"`
	Priority           *int       `ddb:"priority"`
	ClientTypes        *string    `ddb:"client_types"`
	NaiveUotEnable     *uint      `ddb:"naive_uot_enable"`
	NaiveUotVersion    *uint      `ddb:"naive_uot_version"`
	CreateTime         *time.Time `ddb:"create_time"`
	UpdateTime         *time.Time `ddb:"update_time"`
}

// ClientPort is the externally reachable connection port. Port remains the
// listener identity used by the Node Agent and all management operations.
func (node Node) ClientPort() uint {
	if node.ExternalPort != nil && *node.ExternalPort != 0 {
		return *node.ExternalPort
	}
	if node.Port == nil {
		return 0
	}
	return *node.Port
}
