package vo

import "time"

type ServerTrafficStatusVo struct {
	NodeServerId      uint      `json:"nodeServerId" ddb:"node_server_id"`
	NodeServerName    string    `json:"nodeServerName" ddb:"node_server_name"`
	Period            string    `json:"period" ddb:"traffic_period"`
	LimitMode         string    `json:"limitMode" ddb:"traffic_limit_mode"`
	WindowStart       time.Time `json:"windowStart"`
	WindowEnd         time.Time `json:"windowEnd"`
	UploadUsed        uint64    `json:"uploadUsed" ddb:"upload_used"`
	DownloadUsed      uint64    `json:"downloadUsed" ddb:"download_used"`
	TotalUsed         uint64    `json:"totalUsed"`
	UploadLimit       uint64    `json:"uploadLimit" ddb:"traffic_upload_limit"`
	DownloadLimit     uint64    `json:"downloadLimit" ddb:"traffic_download_limit"`
	TotalLimit        uint64    `json:"totalLimit" ddb:"traffic_total_limit"`
	UploadRemaining   uint64    `json:"uploadRemaining"`
	DownloadRemaining uint64    `json:"downloadRemaining"`
	TotalRemaining    uint64    `json:"totalRemaining"`
	Reached           bool      `json:"reached"`
}

type ServerTrafficUsageVo struct {
	NodeServerId   uint   `json:"nodeServerId" ddb:"node_server_id"`
	NodeServerName string `json:"nodeServerName" ddb:"node_server_name"`
	Upload         uint64 `json:"upload" ddb:"upload"`
	Download       uint64 `json:"download" ddb:"download"`
	Total          uint64 `json:"total" ddb:"total"`
}

type ServerTrafficUsagePageVo struct {
	BaseVoPage
	Rows []ServerTrafficUsageVo `json:"rows"`
}

type ServerTrafficUserUsageVo struct {
	AccountId uint   `json:"accountId" ddb:"account_id"`
	Username  string `json:"username" ddb:"username"`
	Upload    uint64 `json:"upload" ddb:"upload"`
	Download  uint64 `json:"download" ddb:"download"`
	Total     uint64 `json:"total" ddb:"total"`
}

type ServerTrafficUserUsagePageVo struct {
	BaseVoPage
	Rows []ServerTrafficUserUsageVo `json:"rows"`
}

type ResetNodeServerTrafficVo struct {
	DeletedRows int64 `json:"deletedRows"`
}
