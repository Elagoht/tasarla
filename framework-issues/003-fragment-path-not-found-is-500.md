# 003 — A fragment path answers 500 when its required fragment reports `ErrNotFound`

> **Resolved in collage v0.39.2** (2026-10-01): "A fragment path answers 404 when its
> required fragment is not found." The app is on v0.39.2;
> `TestOutsidersGetNotFoundForBoardsAndCards` now expects 404 from the fragment paths.

- **Type:** bug (a documented guarantee does not hold)
- **Packages:** `github.com/Elagoht/collage` v0.39.0
- **Found while:** Kanban phase 2. `/boards/{id}/columns` and `/boards/{id}/cards/{card}/panel` must answer 404 to a reader outside the board's team (spec §6), as the pages themselves do.

## Documented statement

README, "What it guarantees":

> **"Missing" and "broken" are different.** A data handler that wraps
> `collage.ErrNotFound` produces a 404 and the page's own not-found page; anything
> else produces a 500 and its error page.

docs/actions.md, "A fragment at its own URL":

> `GET /search/results?q=grid` renders that fragment and nothing else — its data
> handler runs, its children are prefetched and rendered, its failure policy applies,
> because it is the same walk started lower down.

## What happens

The same `Required()` fragment whose handler wraps `collage.ErrNotFound` produces:

- **404** when it is rendered as the page's content,
- **500** when it is rendered at its own fragment path.

## Minimal reproduction

```go
// t3/p.html: <p>{{.}}</p>
func TestFragmentPathNotFound(t *testing.T) {
	app, _ := collage.New(&collage.Config{
		Template: collage.TemplateConfig{Root: "t3", Extension: ".html"},
		Security: collage.SecurityConfig{CSRFKey: []byte(strings.Repeat("k", 32))},
	})
	missing := collage.NewFragment("missing", "p.html").
		WithDataHandler(collage.Load(func(_ context.Context, rc *collage.RenderContext) (string, error) {
			return "", fmt.Errorf("thing %s: %w", rc.Param("id"), collage.ErrNotFound)
		})).Required().Build()
	app.RegisterPage(collage.NewPage("p").WithPath("en", "/things/{id}").WithContent(missing).
		WithFragmentPath("en", "/things/{id}/part", missing).Build())

	h := app.Handler()
	// GET /things/1      = 404
	// GET /things/1/part = 500
}
```

## Expected

`GET /things/1/part` answers 404. A fragment path answers `text/html` without a layout, so the body may be the not-found page's content fragment, or empty. A 500 tells a script (collage-live marks the element stale and retries) and a log that something broke, when the thing is only missing or not the reader's to see.

## Impact on the application

No workaround applied. A reader outside a board's team gets 500, not 404, from `/boards/{id}/columns` and `/boards/{id}/cards/{card}/panel`. Nothing leaks: an existing board and a non-existent one both answer 500, and no content is sent. The test `TestOutsidersGetNotFoundForBoardsAndCards` skips the two fragment URLs with a reference to this issue; remove the skip once the fix lands.
