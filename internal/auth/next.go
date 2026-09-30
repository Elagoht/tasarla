package auth

import (
	"net/url"
	"strings"
)

// SafeNext returns next when it is a path on this site, and "/" otherwise, so
// a link cannot use sign-in to send a reader somewhere else.
func SafeNext(next string) string {
	if next == "" || len(next) > 512 || !strings.HasPrefix(next, "/") ||
		strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n\t") {
		return "/"
	}
	for _, r := range next {
		if r < 0x20 || r == 0x7f {
			return "/"
		}
	}
	// An escaped slash or backslash is still a path on this site, but a link
	// that decodes to //host is not worth the doubt.
	decoded, err := url.PathUnescape(next)
	if err != nil || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "\\") {
		return "/"
	}
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return "/"
	}
	return next
}
