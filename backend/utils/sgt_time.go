package utils

import "time"

// sgtLocation is Singapore Time (UTC+8, no DST). Falls back to a fixed
// UTC+8 zone if the "Asia/Singapore" tzdata entry isn't available in the
// runtime environment, so weekId computation stays deterministic.
var sgtLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Singapore"); err == nil {
		return loc
	}
	return time.FixedZone("SGT", 8*60*60)
}()

// NowSGT returns the current time in Singapore Time (UTC+8).
func NowSGT() time.Time {
	return time.Now().In(sgtLocation)
}

// ISOYearWeekSGT returns the ISO year/week for the current moment, computed
// using Singapore Time so that week boundaries always align with SGT
// (i.e. the ISO week rolls over at Monday 00:00 SGT), regardless of the
// server's local timezone.
func ISOYearWeekSGT() (int, int) {
	return NowSGT().ISOWeek()
}
