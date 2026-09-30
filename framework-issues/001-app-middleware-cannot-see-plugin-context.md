# 001 — Application middleware runs outside plugin middleware, so it cannot read the session

- **Status:** resolved in collage v0.38.0 — plugins in `Config.Plugins` now wrap every `app.Use` middleware. The reproduction below reports `middlewareSessionNil=false` on v0.38.0.

- **Type:** feature request (documented behaviour, but it blocks a documented use)
- **Packages:** `github.com/Elagoht/collage` v0.37.1, `github.com/Elagoht/collage-session` v0.2.1
- **Found while:** planning Kanban phase 1 (`kanban-spec.md` §3, "Oturum sadece kullanıcı id'sini tutar … Bunu bir middleware yapar (`app.Use`)")

## What we want to do

Load the signed-in user from the database once per request, in middleware, and put
the user in the request context for data handlers, actions and `app.Handle` handlers
to read. A user whose `disabled_at` is set must be treated as signed out on the very
next request, whatever their cookie says.

`docs/http.md` presents this as the intended shape: "Whatever it puts in the
request's context is what data handlers read", and the session README says a
handler of your own uses `session.FromContext(r.Context())`.

## What happens

`session.FromContext(r.Context())` is `nil` inside any middleware registered with
`app.Use`. Plugins call `Host.Use` from `Init`, and `Init` runs when the handler is
built (`internal/core/app.go:705`), which is after the application has registered
its own middleware. The chain is one slice composed outermost-first
(`internal/httpx/external.go:79`), so plugin middleware always ends up *inside*
application middleware. `docs/plugins.md` states this ("Wrap every request, after
the application's own middleware"), but nothing tells the application how to run
*after* a plugin.

The application therefore cannot write middleware that depends on anything a plugin
put in the context: session, flash, i18n, secure's nonce.

## Minimal reproduction

```go
func TestAppMiddlewareSeesSession(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	app, _ := collage.New(&collage.Config{
		Template: collage.TemplateConfig{Root: "t", Extension: ".html"}, // t/p.html: <p>{{.}}</p>
		Security: collage.SecurityConfig{CSRFKey: key},
		Plugins:  []collage.Plugin{session.New(session.Options{Key: key})},
	})
	var sawNil bool
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sawNil = session.FromContext(r.Context()) == nil
			next.ServeHTTP(w, r)
		})
	})
	var handlerNil bool
	app.RegisterPage(collage.NewPage("p").WithPath("en", "/").WithContent(
		collage.NewFragment("p", "p.html").WithDataHandler(collage.Load(
			func(ctx context.Context, rc *collage.RenderContext) (string, error) {
				handlerNil = session.Get(rc) == nil
				return "x", nil
			})).Build()).Build())

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	// status=200 middlewareSessionNil=true dataHandlerSessionNil=false
}
```

## Expected

Some supported way for application middleware to run inside plugin middleware,
for example:

- `app.UseAfterPlugins(mw)` / `collage.Config.InnerMiddleware`, composed after every
  plugin's `Host.Use`; or
- plugins initialised (and their `Host.Use` recorded) inside `collage.New` for
  `Config.Plugins`, so an `app.Use` after `New` is inner, with the order documented; or
- a documented ordering rule, e.g. application middleware registered after
  `Start()` goes innermost.

## Possible workarounds (not applied, waiting for a decision)

1. Register a small application-owned `collage.Plugin` after `session.New(…)` in
   `Config.Plugins` and call `host.Use(loadUser)` from its `Init`. This relies on
   plugins being initialised in `Config.Plugins` order, which is not documented.
2. Drop the middleware. Load the user in each data handler and action instead
   (`session.Get(rc)` → DB, shared per render with `collage.Once`), and write a
   custom guard that also rejects disabled users. This duplicates the lookup across
   pages, fragment paths and `app.Handle` handlers, and the spec's
   "one place puts the user in the context" is lost.
