package service

import (
	"encoding/json"
	"testing"

	"trojan-panel/model/vo"
)

func TestServerTrafficSummaryProjectsOnlyDisplayData(t *testing.T) {
	status := vo.ServerTrafficStatusVo{
		NodeServerId:   17,
		NodeServerName: "Selected server",
		Period:         "day",
		LimitMode:      "combined",
		TotalLimit:     500,
		UploadUsed:     120,
		DownloadUsed:   200,
	}
	for _, testcase := range []struct {
		name, period, mode, expected string
	}{
		{"combined", "day", "combined", `{"serverId":17,"serverName":"Selected server","period":"day","totalUsed":320,"totalLimit":500}`},
		{"separate", "month", "separate", `{"serverId":17,"serverName":"Selected server","period":"month","totalUsed":320,"totalLimit":0,"upload":{"used":120,"limit":400},"download":{"used":200,"limit":0}}`},
		{"unlimited", "none", "separate", `{"serverId":17,"serverName":"Selected server","period":"none","totalUsed":320,"totalLimit":0}`},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			status := status
			status.Period, status.LimitMode = testcase.period, testcase.mode
			status.UploadLimit = 400
			hydrateTrafficStatus(&status)
			output, err := json.Marshal(summarizeServerTraffic(status))
			if err != nil || string(output) != testcase.expected {
				t.Fatalf("unexpected summary %s err=%v", output, err)
			}
		})
	}
}
