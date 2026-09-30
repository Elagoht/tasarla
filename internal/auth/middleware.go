package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	session "github.com/Elagoht/collage-session"

	"kanban/internal/store"
)

type userKey struct{}

// WithUser returns ctx carrying u as the signed-in user.
func WithUser(ctx context.Context, u store.User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// UserFrom returns the signed-in user, if there is one.
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userKey{}).(store.User)
	return u, ok
}

// Middleware turns the session's user id into the user, read from the
// database on every request, so a disabled or deleted user is signed out on
// their next request. It must run inside collage-session's middleware, which
// it does when the session plugin is in Config.Plugins (collage v0.38.0+).
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		raw := sess.Get(session.UserKey)
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			sess.Clear()
			next.ServeHTTP(w, r)
			return
		}
		user, err := s.opts.Users.UserByID(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrNotFound), err == nil && user.Disabled():
			sess.Clear()
		case err != nil:
			s.opts.Logger.Error("auth: load signed-in user", "user", id, "err", err)
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		default:
			r = r.WithContext(WithUser(r.Context(), user))
		}
		next.ServeHTTP(w, r)
	})
}
