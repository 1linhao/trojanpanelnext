package middleware

import (
	"github.com/robfig/cron/v3"
	"time"
	"trojan-panel/service"
)

// InitCron 初始化定时任务
func InitCron() {
	location, _ := time.LoadLocation("Asia/Shanghai")
	c := cron.New(cron.WithLocation(location))
	// 定时任务：扫描无效用户 1. 被禁用 2.到期的用户
	_, _ = c.AddFunc("@every 1m", service.CronScanAccounts)
	// 每天12:00发送到期提醒邮件
	_, _ = c.AddFunc("0 0 12 * * *", service.CronScanAccountExpireWarn)
	// 每隔一小时刷新流量排行缓存
	_, _ = c.AddFunc("@every 1h", service.CronTrafficRank)
	// 每月重设除管理员之外的用户下载和上传流量
	_, _ = c.AddFunc("@monthly", service.CronResetDownloadAndUploadMonth)
	// Daily audit retention only; no automatic kernel upgrades are scheduled.
	_, _ = c.AddFunc("0 20 3 * * *", service.CleanupKernelUpgradeAudit)
	service.RecoverKernelTasks()
	c.Start()
}
