package handlers

import (
	"testing"
	"time"
)

func TestProblemRefreshInterval(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		uploadedAt  time.Time
		shouldBlock bool
	}{
		{name: "less than two hours", uploadedAt: now.Add(-problemRefreshInterval + time.Second), shouldBlock: true},
		{name: "exactly two hours", uploadedAt: now.Add(-problemRefreshInterval), shouldBlock: false},
		{name: "more than two hours", uploadedAt: now.Add(-problemRefreshInterval - time.Second), shouldBlock: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked := problemRefreshTooSoon(tt.uploadedAt.Unix(), now)
			if blocked != tt.shouldBlock {
				t.Fatalf("blocked = %v, want %v", blocked, tt.shouldBlock)
			}
		})
	}
}

func TestUnixTimestampSeconds(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  int64
		ok    bool
	}{
		{name: "int32", value: int32(1_700_000_000), want: 1_700_000_000, ok: true},
		{name: "int64", value: int64(1_700_000_000), want: 1_700_000_000, ok: true},
		{name: "whole float", value: float64(1_700_000_000), want: 1_700_000_000, ok: true},
		{name: "fractional float", value: 1_700_000_000.5, ok: false},
		{name: "missing", value: nil, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unixTimestampSeconds(tt.value)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("unixTimestampSeconds() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
