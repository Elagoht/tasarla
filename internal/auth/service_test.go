package auth_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	session "github.com/Elagoht/collage-session"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/auth"
	"kanban/internal/auth/authtest"
	"kanban/internal/store"
	"kanban/internal/webtest"
)

type fakeUsers struct {
	mu     sync.Mutex
	nextID int64
	byKey  map[string]int64
	users  map[int64]store.User
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byKey: map[string]int64{}, users: map[int64]store.User{}}
}

func (f *fakeUsers) UpsertIdentity(_ context.Context, id store.Identity, _ []string, locale string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := id.Issuer + "|" + id.Subject
	uid, ok := f.byKey[key]
	if !ok {
		f.nextID++
		uid = f.nextID
		f.byKey[key] = uid
	}
	u := f.users[uid]
	u.ID, u.Issuer, u.Subject, u.Email, u.Name = uid, id.Issuer, id.Subject, id.Email, id.Name
	if u.Locale == "" {
		u.Locale = locale
	}
	f.users[uid] = u
	return u, nil
}

func (f *fakeUsers) UserByID(_ context.Context, id int64) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) disable(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, u := range f.users {
		if u.Subject == subject {
			now := time.Now()
			u.DisabledAt = &now
			f.users[id] = u
		}
	}
}

type fixture struct {
	users   *fakeUsers
	issuer  *authtest.Issuer
	handler http.Handler
}

func newFixture(t *testing.T, opts authtest.Options) *fixture {
	t.Helper()
	client, issuer := newClient(t, opts)
	users := newFakeUsers()
	key := []byte(strings.Repeat("k", 32))

	app, err := collage.New(&collage.Config{
		Template: collage.TemplateConfig{
			FS: fstest.MapFS{
				"t/private.html": {Data: []byte(`<p>hello {{.Name}}</p><form method="post" action="{{actionURL "logout"}}">{{csrfToken}}</form>`)},
				"t/failed.html":  {Data: []byte(`<p>sign-in failed</p>`)},
			},
			Root: "t", Extension: ".html",
		},
		Locale:   collage.LocaleConfig{Default: "tr", Supported: []string{"tr", "en"}},
		Security: collage.SecurityConfig{CSRFKey: key},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plugins:  []collage.Plugin{session.New(session.Options{Key: key})},
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := collage.NewPage("auth-failed").
		WithContent(collage.NewFragment("auth-failed-content", "failed.html").Build()).Build()
	if err := app.RegisterPage(failed); err != nil {
		t.Fatal(err)
	}
	svc := auth.NewService(auth.Options{
		Client: client, Users: users, DefaultLocale: "tr", Locales: []string{"tr", "en"},
		BaseURL: webtest.Origin, CallbackPath: "/auth/openid/authentik", FailedPage: failed,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for _, a := range svc.Actions() {
		if err := app.RegisterAction(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.Use(svc.Middleware); err != nil {
		t.Fatal(err)
	}
	private := collage.NewFragment("private-content", "private.html").
		WithGuard(session.RequireUser("/login")).
		WithData(collage.Load(func(ctx context.Context, _ *collage.RenderContext) (store.User, error) {
			u, _ := auth.UserFrom(ctx)
			return u, nil
		})).
		Build()
	page := collage.NewPage("private").WithContent(private).
		WithPath("tr", "/private").WithPath("en", "/private").Dynamic().Build()
	if err := app.RegisterPage(page); err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	return &fixture{users: users, issuer: issuer, handler: app.Handler()}
}

var ada = authtest.Grant{Subject: "ada", Email: "ada@example.com", Name: "Ada"}

func TestLoginSendsTheReaderToTheProvider(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	res := webtest.NewBrowser(t, f.handler).Get("/login?next=/private")
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), f.issuer.URL+"/authorize?") {
		t.Fatalf("GET /login = %d %q", res.Status, res.Location())
	}
	u, _ := url.Parse(res.Location())
	if u.Query().Get("code_challenge_method") != "S256" || !strings.Contains(u.Query().Get("scope"), "openid") {
		t.Errorf("authorization request = %v", u.Query())
	}
}

func TestSignInThenOut(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)

	if res := b.Get("/private"); res.Status != http.StatusSeeOther || res.Location() != "/login?next=%2Fprivate" {
		t.Fatalf("anonymous /private = %d %q", res.Status, res.Location())
	}
	res := b.Get("/login?next=/private")
	res = b.Get(f.issuer.Authorize(t, res.Location(), ada))
	if res.Status != http.StatusSeeOther || res.Location() != "/private" {
		t.Fatalf("callback = %d %q, want 303 /private:\n%s", res.Status, res.Location(), res.Body)
	}
	if res := b.Get("/private"); res.Status != http.StatusOK || !strings.Contains(res.Body, "hello Ada") {
		t.Fatalf("signed-in /private = %d:\n%s", res.Status, res.Body)
	}
	if u, err := f.users.UserByID(context.Background(), 1); err != nil || u.Issuer != f.issuer.URL || u.Subject != "ada" {
		t.Errorf("stored user = %+v, %v", u, err)
	}

	res = b.Submit("/private", "/logout", url.Values{})
	if res.Status != http.StatusSeeOther || res.Location() != "/login" {
		t.Fatalf("logout without end_session = %d %q, want 303 /login", res.Status, res.Location())
	}
	if res := b.Get("/private"); res.Status != http.StatusSeeOther {
		t.Fatalf("/private after logout = %d, want a redirect to /login", res.Status)
	}
}

func TestLogoutGoesThroughTheProviderWhenItOffersEndSession(t *testing.T) {
	f := newFixture(t, authtest.Options{EndSession: true})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)

	res := b.Submit("/private", "/logout", url.Values{})
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), f.issuer.EndSessionURL()+"?") {
		t.Fatalf("logout = %d %q", res.Status, res.Location())
	}
	u, _ := url.Parse(res.Location())
	if u.Query().Get("post_logout_redirect_uri") != webtest.Origin+"/login" {
		t.Errorf("post_logout_redirect_uri = %q", u.Query().Get("post_logout_redirect_uri"))
	}
}

func TestNextCannotLeaveTheSite(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	res := b.Get("/login?next=" + url.QueryEscape("//evil.example/x"))
	res = b.Get(f.issuer.Authorize(t, res.Location(), ada))
	if res.Location() != "/" {
		t.Fatalf("callback redirected to %q, want /", res.Location())
	}
}

func TestCallbackRefusals(t *testing.T) {
	cases := map[string]func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response{
		"wrong state": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			cb := f.issuer.Authorize(t, res.Location(), ada)
			return b.Get(strings.Replace(cb, "state=", "state=x", 1))
		},
		"no flow in the session": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			other := webtest.NewBrowser(t, f.handler)
			res := other.Get("/login")
			return b.Get(f.issuer.Authorize(t, res.Location(), ada)) // b never visited /login
		},
		"wrong nonce": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			return b.Get(f.issuer.Authorize(t, res.Location(), authtest.Grant{Subject: "ada", Email: "ada@example.com", Nonce: "evil"}))
		},
		"wrong audience": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			return b.Get(f.issuer.Authorize(t, res.Location(), authtest.Grant{Subject: "ada", Email: "ada@example.com", Audience: "other"}))
		},
		"provider error": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			u, _ := url.Parse(res.Location())
			return b.Get("/auth/openid/authentik?error=access_denied&state=" + url.QueryEscape(u.Query().Get("state")))
		},
		"replayed callback": func(t *testing.T, f *fixture, b *webtest.Browser) webtest.Response {
			res := b.Get("/login")
			cb := f.issuer.Authorize(t, res.Location(), ada)
			if first := b.Get(cb); first.Status != http.StatusSeeOther {
				t.Fatalf("first callback = %d", first.Status)
			}
			b.Submit("/private", "/logout", url.Values{})
			return b.Get(cb)
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, authtest.Options{})
			b := webtest.NewBrowser(t, f.handler)
			res := run(t, f, b)
			if res.Status != http.StatusBadRequest || !strings.Contains(res.Body, "sign-in failed") {
				t.Fatalf("callback = %d, want 400 with the failure page:\n%s", res.Status, res.Body)
			}
			if res := b.Get("/private"); res.Status != http.StatusSeeOther {
				t.Fatalf("/private after a refused callback = %d, want a redirect", res.Status)
			}
		})
	}
}

func TestADisabledUserIsSignedOutOnTheNextRequest(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)
	if res := b.Get("/private"); res.Status != http.StatusOK {
		t.Fatalf("/private = %d", res.Status)
	}

	f.users.disable("ada")

	res := b.Get("/private")
	if res.Status != http.StatusSeeOther || !strings.HasPrefix(res.Location(), "/login") {
		t.Fatalf("/private for a disabled user = %d %q", res.Status, res.Location())
	}
	if b.HasCookie("collage_session") {
		t.Error("the session cookie was kept")
	}
}

func TestADisabledUserCannotSignIn(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)
	f.users.disable("ada")

	fresh := webtest.NewBrowser(t, f.handler)
	res := fresh.Get("/login")
	res = fresh.Get(f.issuer.Authorize(t, res.Location(), ada))
	if res.Status != http.StatusBadRequest {
		t.Fatalf("callback for a disabled user = %d, want 400", res.Status)
	}
}

func TestLoginWhileSignedInGoesToNext(t *testing.T) {
	f := newFixture(t, authtest.Options{})
	b := webtest.NewBrowser(t, f.handler)
	webtest.SignIn(t, b, f.issuer, ada)
	res := b.Get("/login?next=/private")
	if res.Status != http.StatusSeeOther || res.Location() != "/private" {
		t.Fatalf("GET /login signed in = %d %q", res.Status, res.Location())
	}
}
