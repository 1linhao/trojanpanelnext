package model

import "time"

// Account 账户
type Account struct {
	Id                 *uint      `ddb:"id" json:"id"`
	Username           *string    `ddb:"username" json:"username"`
	Pass               *string    `ddb:"pass" json:"pass"`
	Hash               *string    `ddb:"hash" json:"hash"`
	RoleId             *uint      `ddb:"role_id" json:"roleId"`
	Email              *string    `ddb:"email" json:"email"`
	PresetExpire       *uint      `ddb:"preset_expire" json:"presetExpire"`
	PresetQuota        *int       `ddb:"preset_quota" json:"presetQuota"`
	LastLoginTime      *uint      `ddb:"last_login_time" json:"lastLoginTime"`
	ExpireTime         *uint      `ddb:"expire_time" json:"expireTime"`
	Deleted            *uint      `ddb:"deleted" json:"deleted"`
	Quota              *int       `ddb:"quota" json:"quota"`
	Download           *int       `ddb:"download" json:"download"`
	Upload             *int       `ddb:"upload" json:"upload"`
	IpLimit            *uint      `ddb:"ip_limit" json:"ipLimit"`
	UploadSpeedLimit   *uint      `ddb:"upload_speed_limit" json:"uploadSpeedLimit"`
	DownloadSpeedLimit *uint      `ddb:"download_speed_limit" json:"downloadSpeedLimit"`
	CreateTime         *time.Time `ddb:"create_time" json:"createTime"`
	UpdateTime         *time.Time `ddb:"update_time" json:"updateTime"`
}
