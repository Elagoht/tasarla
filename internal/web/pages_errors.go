package web

import (
	"context"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"
)

// publicPage is a page with no path of its own, drawn in the base layout only:
// error pages and the sign-in failure.
func publicPage(name, template, titleKey string) *collage.Page {
	content := collage.NewFragment(name+"-content", template).
		WithData(collage.Effect(func(_ context.Context, rc *collage.RenderContext) error {
			rc.HoistTitle(i18n.T(rc, titleKey))
			return nil
		})).
		Build()
	return collage.NewPage(name).WithLayouts(baseLayout()).WithContent(content).Build()
}

func notFoundPage() *collage.Page {
	return publicPage("not-found", "pages/not_found.html", "errors.not_found.title")
}

func errorPage() *collage.Page {
	return publicPage("server-error", "pages/error.html", "errors.server.title")
}

func authFailedPage() *collage.Page {
	return publicPage("auth-failed", "pages/auth_failed.html", "auth.failed.title")
}
