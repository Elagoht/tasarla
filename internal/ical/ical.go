// Package ical writes iCalendar (RFC 5545) documents of all-day events.
package ical

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var textEscaper = strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", "")

// Event is an all-day event: Start to End, both dates, End inclusive.
type Event struct {
	UID         string
	Start, End  time.Time
	Summary     string
	Description string
	URL         string
	Modified    time.Time
	Priority    int // 1–9; 0: none
}

// Calendar is a published calendar of events.
type Calendar struct {
	Name   string
	Events []Event
}

// Encode writes c as an iCalendar document: CRLF line ends, lines folded at
// 75 octets without splitting a character.
func Encode(c Calendar) []byte {
	var b strings.Builder
	line := func(s string) { writeFolded(&b, s) }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Kanban//Kanban//TR")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	if c.Name != "" {
		line("X-WR-CALNAME:" + textEscaper.Replace(c.Name))
	}
	for _, e := range c.Events {
		line("BEGIN:VEVENT")
		line("UID:" + e.UID)
		line("DTSTAMP:" + stamp(e.Modified))
		line("DTSTART;VALUE=DATE:" + e.Start.Format("20060102"))
		line("DTEND;VALUE=DATE:" + e.End.AddDate(0, 0, 1).Format("20060102"))
		if e.Summary != "" {
			line("SUMMARY:" + textEscaper.Replace(e.Summary))
		}
		if e.Description != "" {
			line("DESCRIPTION:" + textEscaper.Replace(e.Description))
		}
		if e.URL != "" {
			line("URL:" + e.URL)
		}
		if !e.Modified.IsZero() {
			line("LAST-MODIFIED:" + stamp(e.Modified))
		}
		if e.Priority > 0 {
			line("PRIORITY:" + strconv.Itoa(e.Priority))
		}
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return []byte(b.String())
}

func stamp(t time.Time) string {
	if t.IsZero() {
		t = time.Unix(0, 0)
	}
	return t.UTC().Format("20060102T150405Z")
}

// writeFolded writes s as one content line: the first 75 octets, then
// continuation lines of a space and up to 74 more, never inside a character.
func writeFolded(b *strings.Builder, s string) {
	limit := 75
	for len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		if cut == 0 {
			cut = limit
		}
		b.WriteString(s[:cut])
		b.WriteString("\r\n ")
		s = s[cut:]
		limit = 74
	}
	b.WriteString(s)
	b.WriteString("\r\n")
}
