package bo

import "time"

type NodeBo struct {
	Id                 uint      `json:"id"`
	NodeServerId       uint      `json:"nodeServerId"`
	NodeSubId          uint      `json:"nodeSubId"`
	NodeTypeId         uint      `json:"nodeTypeId"`
	Name               string    `json:"name"`
	NodeServerIp       string    `json:"nodeServerIp"`
	NodeServerGrpcPort uint      `json:"nodeServerGrpcPort"`
	GrpcTLSMode        string    `json:"grpcTlsMode"`
	GrpcTLSServerName  string    `json:"grpcTlsServerName"`
	Domain             string    `json:"domain"`
	Port               uint      `json:"port"`
	Priority           int       `json:"priority"`
	Clients            []string  `json:"clients"`
	NaiveUotEnable     uint      `json:"naiveUotEnable"`
	NaiveUotVersion    uint      `json:"naiveUotVersion"`
	CreateTime         time.Time `json:"createTime"`

	Status int `json:"status"`
}
