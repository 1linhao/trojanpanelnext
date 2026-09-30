package middleware

import (
	"github.com/robfig/cron/v3"
	"time"
	"trojan-panel-core/app"
	"trojan-panel-core/app/naiveproxy"
)

// InitCron initialize scheduled tasks
func InitCron() {
	location, _ := time.LoadLocation("Asia/Shanghai")
	c := cron.New(cron.WithLocation(location))
	_, _ = c.AddFunc("@every 1m", naiveproxy.MaintainCertificates)
	_, _ = c.AddFunc("@every 50s", app.CronHandlerUser)
	_, _ = c.AddFunc("@every 10s", app.CronHandlerDownloadAndUpload)
	_, _ = c.AddFunc("@every 10s", app.ReconcileTrafficQuota)
	c.Start()
}
