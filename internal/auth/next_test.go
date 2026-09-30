package auth_test

import (
	"testing"

	"kanban/internal/auth"
)

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                       "/",
		"/":                      "/",
		"/teams/3":               "/teams/3",
		"/en/teams?tab=members":  "/en/teams?tab=members",
		"//evil.com":             "/",
		"/\\evil.com":            "/",
		"https://evil.com/x":     "/",
		"evil.com":               "/",
		"/%2F%2Fevil.com":        "/%2F%2Fevil.com", // a path on this site, not a host
		"/ok\r\nSet-Cookie: x=1": "/",
		"javascript:alert(1)":    "/",
	}
	for in, want := range cases {
		if got := auth.SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
	long := "/" + string(make([]byte, 600))
	if got := auth.SafeNext(long); got != "/" {
		t.Errorf("an overlong next was kept")
	}
}
