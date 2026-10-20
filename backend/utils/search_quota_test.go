package utils

import (
	"testing"
	"time"
)

func TestSearchQuotaLimitsForScore(t *testing.T) {
	for _, tc := range []struct {
		score float64
		want  SearchQuotaLimits
	}{
		{100, SearchQuotaLimits{75, 1000}},
		{75, SearchQuotaLimits{75, 1000}},
		{74.9, SearchQuotaLimits{60, 800}},
		{50, SearchQuotaLimits{60, 800}},
		{49.9, SearchQuotaLimits{45, 600}},
		{40, SearchQuotaLimits{45, 600}},
		{39.9, SearchQuotaLimits{30, 300}},
		{20, SearchQuotaLimits{30, 300}},
		{19.9, SearchQuotaLimits{15, 180}},
		{0, SearchQuotaLimits{15, 180}},
		{-5, SearchQuotaLimits{15, 180}},
	} {
		if got := SearchQuotaLimitsForScore(tc.score); got != tc.want {
			t.Errorf("score %v: got %+v, want %+v", tc.score, got, tc.want)
		}
	}
}

func TestWeekStartSGT(t *testing.T) {
	// Sunday 2026-10-18 23:59 SGT belongs to the week of Monday 2026-10-12.
	sun := time.Date(2026, time.October, 18, 23, 59, 0, 0, sgtLocation)
	if got := WeekStartSGT(sun); !got.Equal(time.Date(2026, time.October, 12, 0, 0, 0, 0, sgtLocation)) {
		t.Fatalf("sunday: got %v", got)
	}
	// Monday 00:00 SGT starts a new week; 2026-10-18 16:00 UTC is exactly that.
	mon := time.Date(2026, time.October, 18, 16, 0, 0, 0, time.UTC)
	if got := WeekStartSGT(mon); !got.Equal(time.Date(2026, time.October, 19, 0, 0, 0, 0, sgtLocation)) {
		t.Fatalf("monday 00:00: got %v", got)
	}
	if got := WeekStartSGT(mon.Add(-time.Second)); !got.Equal(time.Date(2026, time.October, 12, 0, 0, 0, 0, sgtLocation)) {
		t.Fatalf("sunday 23:59:59: got %v", got)
	}
}

func TestBuildSearchQuotaStatus(t *testing.T) {
	limits := SearchQuotaLimits{FiveHour: 15, Weekly: 180}
	now := time.Date(2026, time.October, 15, 12, 0, 0, 0, sgtLocation) // Thursday
	weekStart := WeekStartSGT(now)

	t.Run("no document", func(t *testing.T) {
		s := buildSearchQuotaStatus(nil, limits, 0, now)
		if s.Blocked || s.FiveHour.Used != 0 || s.FiveHour.Remaining != 15 || s.Weekly.Remaining != 180 {
			t.Fatalf("unexpected status %+v", s)
		}
		if s.FiveHour.ResetAt != nil {
			t.Fatal("one-hour window should not be started")
		}
		if *s.NextResetAtUnix != weekStart.AddDate(0, 0, 7).Unix() {
			t.Fatal("next reset should be the weekly reset")
		}
	})

	t.Run("active window", func(t *testing.T) {
		start := now.Add(-30 * time.Minute)
		doc := &searchQuotaDoc{FiveHourStart: start.Unix(), FiveHourUsed: 4, WeekStart: weekStart.Unix(), WeekUsed: 40}
		s := buildSearchQuotaStatus(doc, limits, 0, now)
		if s.FiveHour.Used != 4 || s.Weekly.Used != 40 || s.Blocked {
			t.Fatalf("unexpected status %+v", s)
		}
		if *s.NextResetAtUnix != start.Add(time.Hour).Unix() {
			t.Fatal("next reset should be the one-hour reset")
		}
	})

	t.Run("one-hour exhausted", func(t *testing.T) {
		start := now.Add(-30 * time.Minute)
		doc := &searchQuotaDoc{FiveHourStart: start.Unix(), FiveHourUsed: 15, WeekStart: weekStart.Unix(), WeekUsed: 15}
		s := buildSearchQuotaStatus(doc, limits, 0, now)
		if !s.Blocked || *s.UnlockAtUnix != start.Add(time.Hour).Unix() {
			t.Fatalf("unexpected status %+v", s)
		}
	})

	t.Run("weekly exhausted unlocks next Monday", func(t *testing.T) {
		doc := &searchQuotaDoc{FiveHourStart: now.Add(-time.Hour).Unix(), FiveHourUsed: 1, WeekStart: weekStart.Unix(), WeekUsed: 180}
		s := buildSearchQuotaStatus(doc, limits, 0, now)
		if !s.Blocked || *s.UnlockAtUnix != weekStart.AddDate(0, 0, 7).Unix() {
			t.Fatalf("unexpected status %+v", s)
		}
	})

	for _, elapsed := range []time.Duration{time.Hour - time.Second, time.Hour, 2 * time.Hour} {
		t.Run("window age "+elapsed.String(), func(t *testing.T) {
			start := now.Add(-elapsed)
			doc := &searchQuotaDoc{
				FiveHourStart: start.Unix(), FiveHourUsed: 15,
				WeekStart: weekStart.Unix(), WeekUsed: 40,
			}
			s := buildSearchQuotaStatus(doc, limits, 0, now)
			if elapsed < time.Hour {
				if !s.Blocked || s.FiveHour.Used != 15 || s.UnlockAtUnix == nil || *s.UnlockAtUnix != start.Add(time.Hour).Unix() {
					t.Fatalf("quota should stay exhausted until one hour: %+v", s)
				}
			} else {
				if s.Blocked || s.FiveHour.Used != 0 || s.FiveHour.Remaining != 15 || s.FiveHour.ResetAt != nil || s.UnlockAtUnix != nil {
					t.Fatalf("hourly quota should reset after one hour: %+v", s)
				}
				if s.NextResetAtUnix == nil || *s.NextResetAtUnix != weekStart.AddDate(0, 0, 7).Unix() {
					t.Fatalf("idle window should wait for next search: %+v", s)
				}
			}
			if s.Weekly.Used != 40 || s.Weekly.Remaining != 140 {
				t.Fatalf("hourly reset must preserve weekly usage: %+v", s.Weekly)
			}
		})
	}

	t.Run("expired window and old week reset", func(t *testing.T) {
		doc := &searchQuotaDoc{
			FiveHourStart: now.Add(-6 * time.Hour).Unix(), FiveHourUsed: 15,
			WeekStart: weekStart.AddDate(0, 0, -7).Unix(), WeekUsed: 180,
		}
		s := buildSearchQuotaStatus(doc, limits, 0, now)
		if s.Blocked || s.FiveHour.Used != 0 || s.Weekly.Used != 0 {
			t.Fatalf("unexpected status %+v", s)
		}
	})
}

func TestSGTLocationIsUTCPlusEight(t *testing.T) {
	utc := time.Date(2026, time.September, 24, 15, 59, 0, 0, time.UTC)
	if got := utc.In(sgtLocation).Format("2006-01-02"); got != "2026-09-24" {
		t.Fatalf("before midnight: got %s", got)
	}
	if got := utc.Add(time.Minute).In(sgtLocation).Format("2006-01-02"); got != "2026-09-25" {
		t.Fatalf("at midnight: got %s", got)
	}
}
