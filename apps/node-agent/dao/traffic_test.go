package dao

import (
	"testing"
	"trojan-panel-core/model/bo"
)

func TestQuotaReached(t *testing.T) {
	tests := []struct {
		name   string
		status bo.QuotaStatus
		want   bool
	}{
		{"unlimited period", bo.QuotaStatus{Period: "none", TotalLimit: 1, UploadUsed: 1}, false},
		{"combined below", bo.QuotaStatus{Period: "month", Mode: "combined", TotalLimit: 10, UploadUsed: 4, DownloadUsed: 5}, false},
		{"combined exact", bo.QuotaStatus{Period: "month", Mode: "combined", TotalLimit: 10, UploadUsed: 4, DownloadUsed: 6}, true},
		{"separate upload exact", bo.QuotaStatus{Period: "day", Mode: "separate", UploadLimit: 4, UploadUsed: 4}, true},
		{"separate zero unlimited", bo.QuotaStatus{Period: "day", Mode: "separate", UploadLimit: 0, UploadUsed: 99, DownloadLimit: 10, DownloadUsed: 9}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := quotaReached(test.status); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}
