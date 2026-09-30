# 002 — `i18n.T(rc, …)` returns the key itself inside an action

- **Status:** resolved in collage-i18n v0.2.1 (requires collage v0.39.0) — `Init` now
  leaves the plugin in every request's context with middleware, and `T` looks there
  when a render's shared data has none, so it translates in an action as well as a
  data handler. Pinned by `TestT_TranslatesInAnAction`. The chosen approach is the
  issue's own "middleware putting it in the context": `BeforeActionEvent` carries no
  `RenderContext`, so a `BeforeActionHook` could not seed the action's shared data.

- **Type:** bug (a documented use does not work)
- **Packages:** `github.com/Elagoht/collage-i18n` v0.2.0, with `github.com/Elagoht/collage` v0.38.0 and `github.com/Elagoht/collage-validate` v0.1.3
- **Found while:** Kanban phase 1, Task 7. Team forms translate flash messages and validation messages in the action that handles the post.

## Documented statement

collage-validate README, "Messages":

> 1. `.Message("…")` after a check replaces that check's message, when it failed —
>    for one form, or for a translation of your own:
>    `.Required().Message(i18n.T(rc, "signup.email.required"))`.

Checks run in the action (`validate.Form(rc)` → `v.Field(…)` → `validate.Refuse`), so this example calls `i18n.T` from an action handler.

## What happens

In an action handler, `i18n.T(rc, key)` returns `key`. The same call in a data handler during the same request returns the translation.

`T` looks the plugin up with `collage.Get[*Plugin](rc, pluginKey)`, and the plugin only puts itself there in `OnBeforeRender` (`i18n.go`, `pluginKey` comment: "where OnBeforeRender leaves the plugin for T"). An action runs before any render, so the lookup misses and `T` falls back to the key. Nothing is reported: in the application this shows up as a flash message reading `teams.created`, and a validation message reading `team.user_not_found`.

## Minimal reproduction

```go
// locales/tr.json: {"hi":"Merhaba"}   locales/en.json: {"hi":"Hello"}   t2/p.html: <p>x</p>
func TestI18nTInAction(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	app, _ := collage.New(&collage.Config{
		Template: collage.TemplateConfig{Root: "t2", Extension: ".html"},
		Locale:   collage.LocaleConfig{Default: "tr", Supported: []string{"tr", "en"}},
		Security: collage.SecurityConfig{CSRFKey: key},
		Plugins:  []collage.Plugin{i18n.New(i18n.Options{FS: os.DirFS("."), Dir: "locales"})},
	})
	var inAction, inHandler string
	app.RegisterPage(collage.NewPage("p").WithPath("tr", "/p").
		WithContent(collage.NewFragment("pc", "p.html").WithDataHandler(collage.Load(
			func(_ context.Context, rc *collage.RenderContext) (string, error) {
				inHandler = i18n.T(rc, "hi")
				return "", nil
			})).Build()).Build())
	app.RegisterAction(collage.NewAction("a").WithPath("tr", "/a").WithMethods(http.MethodPost).WithoutCSRF().
		WithHandler(func(_ context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
			inAction = i18n.T(rc, "hi")
			return collage.NoContent(http.StatusNoContent), nil
		}).Build())

	h := app.Handler()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/p", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/a", nil))
	// data handler: "Merhaba"; action (status 204): "hi"
}
```

The same result holds for an action bound to a page with `WithAction`.

## Expected

`i18n.T(rc, …)` translates in the request's locale wherever collage hands the application a `RenderContext`: data handlers and actions alike. For example, the plugin could make itself reachable from the request (a `BeforeActionHook`, or middleware putting it in the context) rather than only from `OnBeforeRender`.

If translating in actions is intentionally not supported, both READMEs should say so. The i18n README would then name the way to translate in an action, and the validate README example would use it. Either way, a key that could not be translated because the plugin was unreachable should be reported, not silently returned.

## Possible workarounds (not applied, waiting for a decision)

1. Keep the plugin value and call `tr.In(rc.Locale).T(key, …)` in actions. This is documented under "Outside a render", which is not quite what an action is.
2. Put keys, not text, into flash messages and validation failures, and translate them in the template. This does not work for `validate` messages, which the template prints as text.
