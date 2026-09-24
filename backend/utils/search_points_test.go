package utils

import (
	"testing"
	"time"
)

func TestHasDailyFreeSearch(t *testing.T) {
	today := NowSGT().Format("2006-01-02")
	yesterday := NowSGT().AddDate(0, 0, -1).Format("2006-01-02")

	for _, tc := range []struct {
		name string
		day  string
		used int
		want bool
	}{
		{"new account", "", 0, true},
		{"fourteenth search used", today, 14, true},
		{"fifteenth search used", today, 15, false},
		{"new day resets eligibility", yesterday, 15, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasDailyFreeSearch(tc.day, tc.used); got != tc.want {
				t.Fatalf("HasDailyFreeSearch(%q, %d) = %v, want %v", tc.day, tc.used, got, tc.want)
			}
		})
	}
}

func TestDailyFreeSearchDayUsesUTCPlusEight(t *testing.T) {
	utc := time.Date(2026, time.September, 24, 15, 59, 0, 0, time.UTC)
	if got := utc.In(sgtLocation).Format("2006-01-02"); got != "2026-09-24" {
		t.Fatalf("before midnight: got %s", got)
	}
	if got := utc.Add(time.Minute).In(sgtLocation).Format("2006-01-02"); got != "2026-09-25" {
		t.Fatalf("at midnight: got %s", got)
	}
}
