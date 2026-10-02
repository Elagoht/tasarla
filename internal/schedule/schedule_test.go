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
		{"weekly sunday only on fri", Spec{Kind: "weekly", Weekdays: 1 << 6, Hour: 9}, ist, "2026-10-02 12:00", "2026-09-27 09:00"},
		{"weekly same weekday after the time", Spec{Kind: "weekly", Weekdays: 1 << 4, Hour: 9}, ist, "2026-10-02 12:00", "2026-10-02 09:00"},
		{"weekly same weekday before the time", Spec{Kind: "weekly", Weekdays: 1 << 4, Hour: 9}, ist, "2026-10-02 08:00", "2026-09-25 09:00"},
		{"weekly no day", Spec{Kind: "weekly", Hour: 9}, ist, "2026-10-02 12:00", ""},
		{"monthly 31 in february", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2027-03-10 12:00", "2027-02-28 09:00"},
		{"monthly 31 leap february", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2028-03-10 12:00", "2028-02-29 09:00"},
		{"monthly 31 in april", Spec{Kind: "monthly", MonthDay: 31, Hour: 9}, ist, "2026-04-30 10:00", "2026-04-30 09:00"},
		{"monthly before this month's day", Spec{Kind: "monthly", MonthDay: 15, Hour: 9}, ist, "2026-10-02 12:00", "2026-09-15 09:00"},
		{"monthly in january reaches back to december", Spec{Kind: "monthly", MonthDay: 15, Hour: 9}, ist, "2027-01-10 12:00", "2026-12-15 09:00"},
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

// A wall-clock time that happens twice, when the clocks fall back, is its
// first occurrence: 02:30 summer time, not 02:30 winter time.
func TestLatestTakesTheFirstOfARepeatedHour(t *testing.T) {
	ber, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, ber)
	got := Latest(Spec{Kind: "daily", Hour: 2, Minute: 30}, ber, now)
	if want := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %v, want %v (02:30 +0200)", got, want.In(ber))
	}
	if _, offset := got.Zone(); offset != 2*60*60 {
		t.Errorf("offset = %d, want +0200", offset)
	}
	// Just after the first 02:30, the second one is not a new moment.
	between := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC) // 02:00 +0100
	if got := Latest(Spec{Kind: "daily", Hour: 2, Minute: 30}, ber, between); !got.Equal(time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC)) {
		t.Errorf("between the two 02:30s: got %v", got)
	}
}
