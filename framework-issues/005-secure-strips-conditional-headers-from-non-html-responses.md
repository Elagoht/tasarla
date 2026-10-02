# 005 — collage-secure: a CSP strips conditional headers from every request, not only HTML pages

- **Type:** bug (the code goes against a documented statement)
- **Packages:** `github.com/Elagoht/collage-secure` v0.1.4, with `github.com/Elagoht/collage` v0.39.2
- **Found while:** Kanban, Phase 3 (calendar feeds). `/cal/{token}/me.ics` is served by a handler registered with `app.Handle`. It answers `text/calendar` with an `ETag` and should answer `If-None-Match` with `304`, so that calendar apps polling every few minutes do not download the feed again each time.

## What the documentation says

README, the section on nonces:

> A page carrying a nonce is sent with `Cache-Control: no-store` and no `ETag`, and its request is answered unconditionally: a `304` would have the browser keep the page with its old nonce under the new header, and every inline script would be blocked. **Only HTML is held back to do this; an event stream, an image, a JSON document pass straight through.**

## What happens

`secure.go`, `middleware` (v0.1.4, around line 171) does this on every request once `Options.CSP` is set:

```go
r.Header.Del("If-None-Match")
r.Header.Del("If-Modified-Since")
```

It runs before the next handler and before the response's content type is known, so the request headers are gone whatever the answer turns out to be. Any handler behind the plugin therefore never sees a conditional request, including `app.Handle` handlers, `http.ServeContent` (the app's `/files/` attachments) and static mounts. A non-HTML response does not "pass straight through": it can never be a `304`.

## Minimal repro

```go
app, _ := collage.New(&collage.Config{Plugins: []collage.Plugin{
	secure.New(secure.Options{CSP: "default-src 'self'"}),
}})
app.Handle("/feed", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("ETag", `"v1"`)
	if r.Header.Get("If-None-Match") == `"v1"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
}))
app.Start()
req := httptest.NewRequest("GET", "/feed", nil)
req.Header.Set("If-None-Match", `"v1"`)
rec := httptest.NewRecorder()
app.Handler().ServeHTTP(rec, req)
// Expected 304; observed 200 with the body. The handler sees no If-None-Match.
```

In the app: `internal/web/calendar_test.go`, `TestCalendarFeeds`, the `If-None-Match` step (uncommitted work on branch `feat/ical`).

## What would help

Any one of these:

1. **Keep the request's headers and decide on the response:** hold the conditional headers aside, and strip them (or turn a `304` into a full answer) only when the response is HTML that carries the nonce marker. This matches the README.
2. **Strip only for collage pages:** leave requests to `app.Handle` and `app.Mount` routes alone.
3. **An option:** for example `Options.Unconditional func(*http.Request) bool`, so an application can name the paths that are revalidated as usual.

## Where the app stands

- Phase 3 calendar feeds: Tasks 1–4 done and committed on `feat/ical`. TestCalendarFeeds passes all checks except the If-None-Match step (304 response). Every other check in the feed test passes: feeds, 404s, `405` with `Allow: GET, HEAD`, removed member.
- No workaround was written. Possible ones, not applied: dropping the `304` (an `ETag` only), or registering `/cal/` outside collage.
