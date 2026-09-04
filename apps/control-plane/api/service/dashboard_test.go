package service

import (
	"testing"
	"time"
)

func TestTrafficRankRange(t *testing.T) {
	now := time.Date(2026, time.August, 24, 4, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		period    string
		value     string
		wantStart string
		wantEnd   string
		wantErr   bool
	}{
		{name: "total", period: "total"},
		{name: "total rejects date", period: "total", value: "2026-08-24", wantErr: true},
		{name: "selected day", period: "day", value: "2024-02-29", wantStart: "2024-02-29", wantEnd: "2024-03-01"},
		{name: "day defaults to Shanghai today", period: "day", wantStart: "2026-08-24", wantEnd: "2026-08-25"},
		{name: "invalid day", period: "day", value: "2026-02-30", wantErr: true},
		{name: "selected month", period: "month", value: "2025-12", wantStart: "2025-12-01", wantEnd: "2026-01-01"},
		{name: "month defaults to Shanghai month", period: "month", wantStart: "2026-08-01", wantEnd: "2026-09-01"},
		{name: "invalid month", period: "month", value: "2026-13", wantErr: true},
		{name: "invalid period", period: "week", value: "2026-08-24", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := trafficDateRange(test.period, test.value, now)
			if (err != nil) != test.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, test.wantErr)
			}
			if start != test.wantStart || end != test.wantEnd {
				t.Fatalf("range=(%q,%q) want=(%q,%q)", start, end, test.wantStart, test.wantEnd)
			}
		})
	}
}
