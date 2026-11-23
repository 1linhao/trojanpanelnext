package model

import "time"

// Account 账户
type Account struct {
	SessionVersion      uint64     `ddb:"session_version" json:"-"`
	Id                  *uint      `ddb:"id" json:"id"`
	Username            *string    `ddb:"username" json:"username"`
	Pass                *string    `ddb:"pass" json:"pass"`
	LegacyLoginVerifier *string    `ddb:"login_verifier" json:"loginVerifier,omitempty"`
	Hash                *string    `ddb:"hash" json:"hash"`
	RoleId              *uint      `ddb:"role_id" json:"roleId"`
	Email               *string    `ddb:"email" json:"email"`
	Remark              *string    `ddb:"remark" json:"-"`
	PresetExpire        *uint      `ddb:"preset_expire" json:"presetExpire"`
	PresetQuota         *int       `ddb:"preset_quota" json:"presetQuota"`
	LastLoginTime       *uint      `ddb:"last_login_time" json:"lastLoginTime"`
	ExpireTime          *uint      `ddb:"expire_time" json:"expireTime"`
	Deleted             *uint      `ddb:"deleted" json:"deleted"`
	Quota               *int       `ddb:"quota" json:"quota"`
	Download            *int       `ddb:"download" json:"download"`
	Upload              *int       `ddb:"upload" json:"upload"`
	IpLimit             *uint      `ddb:"ip_limit" json:"ipLimit"`
	UploadSpeedLimit    *uint      `ddb:"upload_speed_limit" json:"uploadSpeedLimit"`
	DownloadSpeedLimit  *uint      `ddb:"download_speed_limit" json:"downloadSpeedLimit"`
	CreateTime          *time.Time `ddb:"create_time" json:"createTime"`
	UpdateTime          *time.Time `ddb:"update_time" json:"updateTime"`
}

// LoginVerifier preserves the original login verifier for legacy accounts whose
// duplicate connection credential was retired during migration.
func (a Account) LoginVerifier() string {
	if a.LegacyLoginVerifier != nil && *a.LegacyLoginVerifier != "" {
		return *a.LegacyLoginVerifier
	}
	if a.Pass != nil {
		return *a.Pass
	}
	return ""
}
