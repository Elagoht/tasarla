package web

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Elagoht/collage/pkg/collage"
)

// maxCount bounds every limit and count a form sets: WIP limits, the person
// limit, the attachments a condition asks for. It keeps them far inside the
// database's integer columns.
const maxCount = 10000

// maxEstimate bounds a card's estimate.
const maxEstimate = 100000

// badText reports whether any value of the posted form, or a file's name, is
// not text the database can hold: invalid UTF-8 or a NUL byte. Such a request
// was not sent by the app's forms, and is refused before it is read.
func badText(rc *collage.RenderContext) bool {
	bad := func(s string) bool { return !utf8.ValidString(s) || strings.ContainsRune(s, 0) }
	r := rc.Request
	for k, vs := range r.PostForm {
		if bad(k) {
			return true
		}
		for _, v := range vs {
			if bad(v) {
				return true
			}
		}
	}
	if r.MultipartForm != nil {
		for _, fs := range r.MultipartForm.File {
			for _, f := range fs {
				if bad(f.Filename) {
					return true
				}
			}
		}
	}
	return false
}

// parseID reads an id from a URL as it is written there, "3": "+3" and "03"
// would be other addresses of the same page.
func parseID(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil && (n <= 0 || strconv.FormatInt(n, 10) != s) {
		err = strconv.ErrSyntax
	}
	return n, err
}

// boundedCount reads a whole number from 1 to maxCount.
func boundedCount(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil && n > 0 && n <= maxCount
}

// estimatePattern is an estimate as people write it: digits, with a point or
// a comma before the fraction. ParseFloat alone also takes NaN, Inf, hex and
// underscores.
var estimatePattern = regexp.MustCompile(`^[0-9]+([.,][0-9]+)?$`)

// parseEstimate reads an estimate from 0 to maxEstimate.
func parseEstimate(s string) (float64, bool) {
	if !estimatePattern.MatchString(s) {
		return 0, false
	}
	e, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return e, err == nil && e <= maxEstimate
}
