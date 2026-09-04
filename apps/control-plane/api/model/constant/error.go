package constant

// 错误msg
const (
	SysError          string = "system error, please contact the system administrator"
	ValidateFailed    string = "invalid argument"
	UnauthorizedError string = "not authentication"
	IllegalTokenError string = "authentication failed"
	ForbiddenError    string = "permission denied"
	TokenExpiredError string = "login expired"
	LogOutError       string = "you are not logged in"

	NoDeleteSysadmin      string = "cannot delete sysadmin account"
	NoDisableSysadmin     string = "sysadmin account cannot be disabled"
	OriPassError          string = "the original password was entered incorrectly"
	UsernameOrPassError   string = "wrong username or password"
	NodeURLError          string = "url generation failed"
	NodeClientUnsupported string = "the selected node cannot be exported to one or more clients"
	CaptchaError          string = "verification code error"
	CaptchaGenerateError  string = "verification code generation failed"
	LoginLimitError       string = "the password has been entered incorrectly too many times, the account has been locked for 30 minutes, please try again later"

	UsernameExist       string = "username already exists"
	NodeNameExist       string = "node name already exists"
	NodeServerNameExist string = "server name already exists"
	NodeNotExist        string = "the node does not exist"
	NodeTypeNotExist    string = "the node type does not exist"
	RoleNotExist        string = "the role does not exist"
	SystemNotExist      string = "this system setting does not exist"
	FileTaskNotExist    string = "the file task does not exist"
	FileNotExist        string = "the file does not exist"

	AccountRegisterClosed string = "account registration is disabled"
	AccountDisabled       string = "this account has been disabled"
	FileTaskNotSuccess    string = "the task has not been executed yet"

	FileSizeTooBig  string = "the file is too big"
	FileFormatError string = "file format not supported"
	FileUploadError string = "file upload failed"

	RowNotEnough string = "data is empty"

	SystemEmailError string = "system mailbox is not set"

	BlackListError      string = "because your recent abnormal operations are too frequent, access has been restricted. If you need to cancel the restriction, please contact the administrator"
	RateLimiterError    string = "order too fast"
	TelegramBotApiError string = "failed to initialize Telegram Bot Api"

	GrpcError        string = "the remote service connection failed, please check the remote service configuration"
	GrpcAddNodeError string = "remote service failed to add node, please try again later"
	LoadKeyPairError string = "failed to load local key and certificate"

	PortIsOccupied         string = "the port is occupied, please check the port or choose another port"
	PortRangeError         string = "the port range is between 100-30000"
	NodeServerDeletedError string = "there are nodes under this server"
)
