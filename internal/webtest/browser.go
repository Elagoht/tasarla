// Package webtest drives the application through its handler the way a
// browser would: it keeps cookies and carries forgery tokens.
package webtest

import (
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"kanban/internal/auth/authtest"
)

// Origin is the scheme and host every request is made to.
const Origin = "http://kanban.test"

// Response is what one request was answered with.
type Response struct {
	Status int
	Header http.Header
	Body   string
}

// Location is the redirect target, if any.
func (r Response) Location() string { return r.Header.Get("Location") }

// Browser keeps the cookies a site sets between requests.
type Browser struct {
	t       testing.TB
	handler http.Handler
	cookies map[string]string
}

// NewBrowser returns a browser with no cookies.
func NewBrowser(t testing.TB, h http.Handler) *Browser {
	return &Browser{t: t, handler: h, cookies: map[string]string{}}
}

// Get requests path, without following redirects.
func (b *Browser) Get(path string) Response {
	return b.do(httptest.NewRequest(http.MethodGet, Origin+path, nil))
}

// Post submits form to path from this site's own origin.
func (b *Browser) Post(path string, form url.Values) Response {
	req := httptest.NewRequest(http.MethodPost, Origin+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", Origin)
	return b.do(req)
}

// Submit loads page, copies its forgery token into form and posts it to action.
func (b *Browser) Submit(page, action string, form url.Values) Response {
	b.t.Helper()
	res := b.Get(page)
	if res.Status != http.StatusOK {
		b.t.Fatalf("webtest: GET %s = %d, want 200", page, res.Status)
	}
	form.Set("_csrf", CSRFToken(b.t, res.Body))
	return b.Post(action, form)
}

// Fetch posts form to path the way board.js and collage-live do: with fetch,
// marked with collage.FetchHeader.
func (b *Browser) Fetch(path string, form url.Values) Response {
	req := httptest.NewRequest(http.MethodPost, Origin+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", Origin)
	req.Header.Set("Collage-Fetch", "1")
	return b.do(req)
}

// SubmitFetch is Submit through Fetch.
func (b *Browser) SubmitFetch(page, action string, form url.Values) Response {
	b.t.Helper()
	res := b.Get(page)
	if res.Status != http.StatusOK {
		b.t.Fatalf("webtest: GET %s = %d, want 200", page, res.Status)
	}
	form.Set("_csrf", CSRFToken(b.t, res.Body))
	return b.Fetch(action, form)
}

// Cookies returns what the browser holds, for a request made outside it.
func (b *Browser) Cookies() []*http.Cookie {
	var out []*http.Cookie
	for name, value := range b.cookies {
		out = append(out, &http.Cookie{Name: name, Value: value})
	}
	return out
}

// HasCookie reports whether the browser holds a cookie named name.
func (b *Browser) HasCookie(name string) bool {
	_, ok := b.cookies[name]
	return ok
}

func (b *Browser) do(req *http.Request) Response {
	b.t.Helper()
	for name, value := range b.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, req)
	res := rec.Result()
	for _, c := range res.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		b.t.Fatal(err)
	}
	return Response{Status: res.StatusCode, Header: res.Header, Body: string(body)}
}

var csrfInput = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

// CSRFToken returns the forgery token in the first form of body.
func CSRFToken(t testing.TB, body string) string {
	t.Helper()
	m := csrfInput.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("webtest: no forgery token in the page:\n%s", body)
	}
	return html.UnescapeString(m[1])
}

// SignIn signs b in through /login and the provider as g.
func SignIn(t testing.TB, b *Browser, issuer *authtest.Issuer, g authtest.Grant) {
	t.Helper()
	res := b.Get("/login")
	if res.Status != http.StatusSeeOther {
		t.Fatalf("webtest: GET /login = %d, want 303", res.Status)
	}
	res = b.Get(issuer.Authorize(t, res.Location(), g))
	if res.Status != http.StatusSeeOther {
		t.Fatalf("webtest: callback = %d, want 303:\n%s", res.Status, res.Body)
	}
}
