package web

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"kanban/internal/auth"
	"kanban/internal/store"
)

// inlineTypes are the content types a browser shows rather than downloads.
// Everything else, SVG included, is an attachment (spec §9).
var inlineTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// filesHandler serves /files/{id}. It runs outside collage's pages, so it
// checks the reader itself: the user the auth middleware put in the context,
// and their access to the attachment's board. Anything else is 404.
func (h *handlers) filesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		user, ok := auth.UserFrom(r.Context())
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/files/"), 10, 64)
		if !ok || err != nil {
			http.NotFound(w, r)
			return
		}
		a, err := h.store.AttachmentFor(r.Context(), id, user)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			h.log.Error("files: load attachment", "id", id, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		f, err := h.files.Open(a.StorageKey)
		if err != nil {
			h.log.Error("files: open", "id", id, "err", err)
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		disposition := "attachment"
		if inlineTypes[a.ContentType] {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", a.ContentType)
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Filename}))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, no-cache")
		http.ServeContent(w, r, "", a.CreatedAt, f)
	})
}
