package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kanban/internal/auth"
	"kanban/internal/auth/authtest"
)

const redirectURL = "http://kanban.test/auth/openid/authentik"

func newClient(t *testing.T, opts authtest.Options) (*auth.Client, *authtest.Issuer) {
	t.Helper()
	opts.RedirectURL = redirectURL
	issuer := authtest.NewIssuer(t, opts)
	client, err := auth.NewClient(context.Background(), auth.ClientConfig{
		Issuer: issuer.URL, ClientID: issuer.ClientID, ClientSecret: issuer.ClientSecret,
		RedirectURL: redirectURL, Scopes: []string{"openid", "email", "profile"},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, issuer
}

// codeFor runs the authorization step and returns the code the provider issued.
func codeFor(t *testing.T, issuer *authtest.Issuer, authURL string, g authtest.Grant) string {
	t.Helper()
	back, err := url.Parse(issuer.Authorize(t, authURL, g))
	if err != nil {
		t.Fatal(err)
	}
	return back.Query().Get("code")
}

func TestExchangeReturnsTheIdentity(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	authURL := client.AuthURL("state-1", "nonce-1", "verifier-verifier-verifier-verifier-verifier")
	code := codeFor(t, issuer, authURL, authtest.Grant{Subject: "sub-1", Email: "ada@example.com", Name: "Ada"})

	id, err := client.Exchange(context.Background(), code, "verifier-verifier-verifier-verifier-verifier", "nonce-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Issuer != issuer.URL || id.Subject != "sub-1" || id.Email != "ada@example.com" || id.Name != "Ada" {
		t.Errorf("identity = %+v", id)
	}
}

func TestExchangeFallsBackToThePreferredUsername(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	v := "verifier-verifier-verifier-verifier-verifier"
	// What Authentik's test provider sends: no name, a username.
	code := codeFor(t, issuer, client.AuthURL("s", "n", v), authtest.Grant{Subject: "s", Email: "e@example.com", PreferredUsername: "elagoht"})
	id, err := client.Exchange(context.Background(), code, v, "n")
	if err != nil {
		t.Fatal(err)
	}
	if id.Name != "elagoht" {
		t.Errorf("Name = %q, want the preferred username", id.Name)
	}
}

func TestExchangeRefusesTamperedTokens(t *testing.T) {
	cases := map[string]authtest.Grant{
		"nonce":    {Subject: "s", Email: "e@example.com", Nonce: "someone-elses"},
		"audience": {Subject: "s", Email: "e@example.com", Audience: "another-client"},
	}
	for name, g := range cases {
		t.Run(name, func(t *testing.T) {
			client, issuer := newClient(t, authtest.Options{})
			v := "verifier-verifier-verifier-verifier-verifier"
			code := codeFor(t, issuer, client.AuthURL("s", "n", v), g)
			if _, err := client.Exchange(context.Background(), code, v, "n"); err == nil {
				t.Fatal("Exchange accepted a tampered ID token")
			} else if name == "nonce" && !errors.Is(err, auth.ErrNonce) {
				t.Errorf("err = %v, want ErrNonce", err)
			}
		})
	}
}

func TestExchangeRefusesAWrongVerifier(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	code := codeFor(t, issuer, client.AuthURL("s", "n", "verifier-verifier-verifier-verifier-verifier"), authtest.Grant{Subject: "s", Email: "e@example.com"})
	if _, err := client.Exchange(context.Background(), code, "another-verifier-another-verifier-another", "n"); err == nil {
		t.Fatal("Exchange succeeded with a verifier that does not match the challenge")
	}
}

func TestExchangeRefusesAMissingEmail(t *testing.T) {
	client, issuer := newClient(t, authtest.Options{})
	v := "verifier-verifier-verifier-verifier-verifier"
	code := codeFor(t, issuer, client.AuthURL("s", "n", v), authtest.Grant{Subject: "s"})
	if _, err := client.Exchange(context.Background(), code, v, "n"); err == nil {
		t.Fatal("Exchange accepted an ID token without email")
	}
}

func TestLogoutURL(t *testing.T) {
	without, _ := newClient(t, authtest.Options{})
	if got := without.LogoutURL("http://kanban.test/login"); got != "" {
		t.Errorf("LogoutURL without end_session_endpoint = %q, want empty", got)
	}
	with, issuer := newClient(t, authtest.Options{EndSession: true})
	got := with.LogoutURL("http://kanban.test/login")
	if !strings.HasPrefix(got, issuer.EndSessionURL()+"?") {
		t.Fatalf("LogoutURL = %q", got)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("client_id") != issuer.ClientID || u.Query().Get("post_logout_redirect_uri") != "http://kanban.test/login" {
		t.Errorf("LogoutURL query = %v", u.Query())
	}
}

func TestNewClientFailsWhenTheIssuerIsUnreachable(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	_, err := auth.NewClient(context.Background(), auth.ClientConfig{Issuer: dead.URL, ClientID: "x", RedirectURL: redirectURL})
	if err == nil {
		t.Fatal("NewClient succeeded against an unreachable issuer")
	}
}
