package vo

// NodeServerRegistrationVo is the exact database identity returned on creation.
type NodeServerRegistrationVo struct {
	Id                uint   `json:"id"`
	Name              string `json:"name"`
	Ip                string `json:"ip"`
	GrpcPort          uint   `json:"grpcPort"`
	GrpcTLSServerName string `json:"grpcTlsServerName"`
}

// NodeDeploymentVo never includes database passwords, Redis passwords or keys.
type NodeDeploymentVo struct {
	NodeServerRegistrationVo
	Version            string `json:"version"`
	WebHost            string `json:"webHost"`
	MariaDBHost        string `json:"mariadbHost"`
	MariaDBPort        int    `json:"mariadbPort"`
	MariaDBUsesWebHost bool   `json:"mariadbUsesWebHost"`
	RedisHost          string `json:"redisHost"`
	RedisPort          int    `json:"redisPort"`
	RedisUsesWebHost   bool   `json:"redisUsesWebHost"`
	DocsURL            string `json:"docsUrl"`
}
