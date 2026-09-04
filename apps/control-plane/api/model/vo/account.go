package vo

import (
	"time"
)

type AccountVo struct {
	Id            uint      `json:"id"`
	Quota         int       `json:"quota"`
	Download      int       `json:"download"`
	Upload        int       `json:"upload"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	RoleId        uint      `json:"roleId"`
	Deleted       uint      `json:"deleted"`
	PresetExpire  uint      `json:"presetExpire"`
	PresetQuota   int       `json:"presetQuota"`
	LastLoginTime uint      `json:"lastLoginTime"`
	ExpireTime    uint      `json:"expireTime"`
	CreateTime    time.Time `json:"createTime"`
	Roles         []string  `json:"roles"`
}

type AccountPageVo struct {
	BaseVoPage
	AccountVos []AccountVo `json:"accounts"`
}

type AccountLoginVo struct {
	Token string `json:"token"`
}

type AccountInfo struct {
	Id       uint     `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

type AccountTrafficRankVo struct {
	Username    string `json:"username" ddb:"username"`
	Upload      uint64 `json:"upload" ddb:"upload"`
	Download    uint64 `json:"download" ddb:"download"`
	TrafficUsed uint64 `json:"trafficUsed" ddb:"traffic_used"`
}

type AccountExportVo struct {
	Username      string    `json:"username" ddb:"username"`
	Pass          string    `json:"pass" ddb:"pass"`
	Hash          string    `json:"hash" ddb:"hash"`
	RoleId        uint      `json:"roleId" ddb:"role_id"`
	Email         string    `json:"email" ddb:"email"`
	PresetExpire  uint      `json:"presetExpire" ddb:"preset_expire"`
	PresetQuota   int       `json:"presetQuota" ddb:"preset_quota"`
	LastLoginTime uint      `json:"lastLoginTime" ddb:"last_login_time"`
	ExpireTime    uint      `json:"expireTime" ddb:"expire_time"`
	Deleted       uint      `json:"deleted" ddb:"deleted"`
	Quota         int       `json:"quota" ddb:"quota"`
	Download      int       `json:"download" ddb:"download"`
	Upload        int       `json:"upload" ddb:"upload"`
	CreateTime    time.Time `json:"createTime" ddb:"create_time"`
}

type CaptureVo struct {
	CaptchaId  string `json:"captchaId"`
	CaptchaImg string `json:"captchaImg"`
}

type AccountUnusedExportVo struct {
	Username string `json:"username" ddb:"username"`
	Pass     string `json:"pass" ddb:"pass"`
}
