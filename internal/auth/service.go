package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"

	session "github.com/Elagoht/collage-session"
	"github.com/Elagoht/collage/pkg/collage"
	"golang.org/x/oauth2"

	"kanban/internal/store"
)

// Users is what signing in needs from the store.
type Users interface {
	UpsertIdentity(ctx context.Context, id store.Identity, adminEmails []string, locale string) (store.User, error)
	UserByID(ctx context.Context, id int64) (store.User, error)
}

// Options configures the sign-in flow.
type Options struct {
	Client        *Client
	Users         Users
	AdminEmails   []string
	DefaultLocale string
	Locales       []string
	BaseURL       string
	CallbackPath  string
	FailedPage    *collage.Page
	Logger        *slog.Logger
}

// Service is sign-in, its callback and sign-out.
type Service struct {
	opts Options
}

// NewService returns the sign-in flow.
func NewService(opts Options) *Service { return &Service{opts: opts} }

// Session keys holding one sign-in attempt between /login and the callback.
const (
	keyState    = "oidc_state"
	keyNonce    = "oidc_nonce"
	keyVerifier = "oidc_verifier"
	keyNext     = "oidc_next"
)

// Actions returns the routes of the flow: GET /login and POST /logout in every
// locale, and the callback at the path of the redirect URI registered with the
// provider, in the default locale only.
func (s *Service) Actions() []*collage.Action {
	login := collage.NewAction("login").WithMethods(http.MethodGet).WithHandler(s.login)
	logout := collage.NewAction("logout").WithMethods(http.MethodPost).WithHandler(s.logout)
	for _, l := range s.opts.Locales {
		login = login.WithPath(l, "/login")
		logout = logout.WithPath(l, "/logout")
	}
	callback := collage.NewAction("auth-callback").WithMethods(http.MethodGet).
		WithPath(s.opts.DefaultLocale, s.opts.CallbackPath).WithHandler(s.callback)
	return []*collage.Action{login.Build(), callback.Build(), logout.Build()}
}

func (s *Service) login(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	next := SafeNext(rc.Request.URL.Query().Get("next"))
	if _, ok := UserFrom(ctx); ok {
		return collage.SeeOther(next), nil
	}
	state, err := randomToken()
	if err != nil {
		return nil, err
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	sess := session.Get(rc)
	for _, kv := range [][2]string{{keyState, state}, {keyNonce, nonce}, {keyVerifier, verifier}, {keyNext, next}} {
		if err := sess.Set(kv[0], kv[1]); err != nil {
			return nil, err
		}
	}
	return collage.SeeOther(s.opts.Client.AuthURL(state, nonce, verifier)), nil
}

func (s *Service) callback(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	sess := session.Get(rc)
	state, nonce, verifier, next := sess.Get(keyState), sess.Get(keyNonce), sess.Get(keyVerifier), sess.Get(keyNext)
	// One attempt per visit to /login: the flow is spent before anything can fail.
	for _, k := range []string{keyState, keyNonce, keyVerifier, keyNext} {
		sess.Delete(k)
	}

	q := rc.Request.URL.Query()
	if state == "" || verifier == "" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
		return s.fail("state does not match this session's sign-in"), nil
	}
	if e := q.Get("error"); e != "" {
		return s.fail("provider answered " + e), nil
	}
	id, err := s.opts.Client.Exchange(ctx, q.Get("code"), verifier, nonce)
	if err != nil {
		return s.fail(err.Error()), nil
	}
	user, err := s.opts.Users.UpsertIdentity(ctx, id, s.opts.AdminEmails, s.opts.DefaultLocale)
	if err != nil {
		return nil, err
	}
	if user.Disabled() {
		return s.fail("user " + strconv.FormatInt(user.ID, 10) + " is disabled"), nil
	}
	sess.Regenerate()
	if err := sess.Set(session.UserKey, strconv.FormatInt(user.ID, 10)); err != nil {
		return nil, err
	}
	return collage.SeeOther(SafeNext(next)), nil
}

func (s *Service) logout(_ context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	session.Get(rc).Clear()
	if u := s.opts.Client.LogoutURL(s.opts.BaseURL + "/login"); u != "" {
		return collage.SeeOther(u), nil
	}
	return collage.SeeOther("/login"), nil
}

func (s *Service) fail(reason string) *collage.ActionResult {
	s.opts.Logger.Warn("auth: sign-in refused", "reason", reason)
	return &collage.ActionResult{Status: http.StatusBadRequest, Page: s.opts.FailedPage}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
