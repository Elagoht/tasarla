package schedule

import (
	"testing"
	"time"
)

func TestLatest(t *testing.T) {
	ist, _ := time.LoadLocation("Europe/Istanbul")
	ber, _ := time.LoadLocation("Europe/Berlin")
	at := func(loc *time.Location, s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		name string
		spec Spec
		loc  *time.Location
		now  string
		want string // "" for zero
	}{
		{"daily before the time", Spec{Kind: "daily", Hour: 9}, ist, "2026-10-02 08:59", "2026-10-01 09:00"},
		{"daily at the time", Spec{Kind: "daily", Hour: 9}, ist, "2026-10-02 09:00", "2026-10-02 09:00"},
		{"weekly mon+thu on fri", Spec{Kind: "weekly", Weekdays: 1 | 1<<3, Hour: 9}, ist, "2026-10-02 12:00", "2026-10-01 09:00"},
		{"weekly no day", Spec{Kind: "weekly", Hour: 9}, ist, "2026-10-02 12:00", ""},
		{"monthly 31 in february", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2027-03-10 12:00", "2027-02-28 09:00"},
		{"monthly 31 leap february", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2028-03-10 12:00", "2028-02-29 09:00"},
		{"monthly 31 in april", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2026-04-30 10:00", "2026-04-30 09:00"},
		{"monthly before this month's day", Spec{Kind: "monthly", MonthDay: 15, Hour: 9}, ist, "2026-10-02 12:00", "2026-09-15 09:00"},
		{"dst gap moves forward", Spec{Kind: "daily", Hour: 2, Minute: 30}, ber, "2026-03-29 12:00", "2026-03-29 03:30"},
		{"unknown kind", Spec{Kind: "hourly"}, ist, "2026-10-02 12:00", ""},
	}
	for _, c := range cases {
		got := Latest(c.spec, c.loc, at(c.loc, c.now))
		if c.want == "" {
			if !got.IsZero() {
				t.Errorf("%s: got %v, want zero", c.name, got)
			}
			continue
		}
		if want := at(c.loc, c.want); !got.Equal(want) {
			t.Errorf("%s: got %v, want %v", c.name, got.In(c.loc), want)
		}
	}
}
