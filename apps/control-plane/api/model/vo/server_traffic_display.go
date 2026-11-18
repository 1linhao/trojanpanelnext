package vo

type ServerTrafficDisplayVo struct {
	Enabled   bool                           `json:"enabled"`
	ServerIDs []uint                         `json:"serverIds"`
	Servers   []ServerTrafficDisplayOptionVo `json:"servers"`
}

type ServerTrafficDisplayOptionVo struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type ServerTrafficSummaryVo struct {
	ServerID   uint                      `json:"serverId"`
	ServerName string                    `json:"serverName"`
	Period     string                    `json:"period"`
	TotalUsed  uint64                    `json:"totalUsed"`
	TotalLimit uint64                    `json:"totalLimit"`
	Upload     *ServerTrafficDirectionVo `json:"upload,omitempty"`
	Download   *ServerTrafficDirectionVo `json:"download,omitempty"`
}

type ServerTrafficDirectionVo struct {
	Used  uint64 `json:"used"`
	Limit uint64 `json:"limit"`
}
