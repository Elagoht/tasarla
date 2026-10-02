package ical

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

func TestEncodeGolden(t *testing.T) {
	got := string(Encode(Calendar{Name: "Kanban — bana atananlar", Events: []Event{{
		UID: "card-7@kanban.test", Start: day("2026-10-05"), End: day("2026-10-09"),
		Summary: "[Sprint] Rapor, taslak; son", Description: "https://kanban.test/boards/1/cards/7\n\nsatır\\iki",
		URL: "https://kanban.test/boards/1/cards/7", Modified: time.Date(2026, 10, 2, 8, 30, 0, 0, time.UTC), Priority: 1,
	}}}))
	want := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Kanban//Kanban//TR",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"X-WR-CALNAME:Kanban — bana atananlar",
		"BEGIN:VEVENT",
		"UID:card-7@kanban.test",
		"DTSTAMP:20261002T083000Z",
		"DTSTART;VALUE=DATE:20261005",
		"DTEND;VALUE=DATE:20261010",
		"SUMMARY:[Sprint] Rapor\\, taslak\\; son",
		"DESCRIPTION:https://kanban.test/boards/1/cards/7\\n\\nsatır\\\\iki",
		"URL:https://kanban.test/boards/1/cards/7",
		"LAST-MODIFIED:20261002T083000Z",
		"PRIORITY:1",
		"END:VEVENT",
		"END:VCALENDAR",
		"",
	}, "\r\n")
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFoldKeepsRunesWhole(t *testing.T) {
	long := strings.Repeat("ş", 100) // 2 bytes each
	out := string(Encode(Calendar{Events: []Event{{UID: "u", Start: day("2026-10-05"), End: day("2026-10-05"), Summary: long}}}))
	var joined strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line of %d octets: %q", len(line), line)
		}
		if !utf8.ValidString(line) {
			t.Errorf("a rune was split: %q", line)
		}
		if strings.HasPrefix(line, " ") {
			joined.WriteString(line[1:])
		} else {
			joined.WriteString("\n" + line)
		}
	}
	if !strings.Contains(joined.String(), "SUMMARY:"+long) {
		t.Error("unfolding does not give the summary back")
	}
}

func TestNoPriorityNoLine(t *testing.T) {
	out := string(Encode(Calendar{Events: []Event{{UID: "u", Start: day("2026-10-05"), End: day("2026-10-05")}}}))
	if strings.Contains(out, "PRIORITY") || strings.Contains(out, "\nURL") || strings.Contains(out, "DESCRIPTION") {
		t.Errorf("empty fields written:\n%s", out)
	}
	if !strings.Contains(out, "DTEND;VALUE=DATE:20261006\r\n") {
		t.Error("a one-day event ends the next day")
	}
}
