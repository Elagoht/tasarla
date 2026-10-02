// Package schedule finds the moments a card template's schedule names.
package schedule

import "time"

// Spec is a schedule: its kind, days and time of day.
type Spec struct {
	Kind     string // "daily", "weekly", "monthly"
	Weekdays uint8  // bit 0 Monday … bit 6 Sunday
	MonthDay int    // 1–31; a shorter month uses its last day
	Hour     int
	Minute   int
}

// Latest is the last moment at or before now that s names, in loc; zero when
// s names none (an unknown kind, or weekly with no day).
func Latest(s Spec, loc *time.Location, now time.Time) time.Time {
	now = now.In(loc)
	y, m, d := now.Date()
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, s.Hour, s.Minute, 0, 0, loc) }
	switch s.Kind {
	case "daily":
		for i := 0; i < 2; i++ {
			if t := at(y, m, d-i); !t.After(now) {
				return t
			}
		}
	case "weekly":
		if s.Weekdays&0x7f == 0 {
			return time.Time{}
		}
		for i := 0; i < 8; i++ {
			t := at(y, m, d-i)
			bit := (int(t.Weekday()) + 6) % 7
			if s.Weekdays&(1<<bit) != 0 && !t.After(now) {
				return t
			}
		}
	case "monthly":
		for i := 0; i < 2; i++ {
			first := time.Date(y, m-time.Month(i), 1, 0, 0, 0, 0, loc)
			last := first.AddDate(0, 1, -1).Day()
			t := at(first.Year(), first.Month(), min(s.MonthDay, last))
			if !t.After(now) {
				return t
			}
		}
	}
	return time.Time{}
}
