// Package authtest is an OpenID Connect provider for tests: discovery, JWKS,
// authorization codes with PKCE, and signed ID tokens.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Options shapes the provider.
type Options struct {
	// RedirectURL is the only redirect_uri the provider accepts.
	RedirectURL string
	// EndSession advertises an end_session_endpoint in discovery.
	EndSession bool
}

// Grant is who signs in at one authorization, and how the ID token is to be
// tampered with, for tests of what a client must refuse.
type Grant struct {
	Subject           string
	Email             string
	Name              string
	PreferredUsername string

	Nonce    string // replaces the nonce the client asked for
	Audience string // replaces the client id in aud
}

// Issuer is a running provider.
type Issuer struct {
	URL          string
	ClientID     string
	ClientSecret string

	opts   Options
	key    *rsa.PrivateKey
	server *httptest.Server

	mu    sync.Mutex
	codes map[string]pending
}

type pending struct {
	grant     Grant
	nonce     string
	challenge string
}

type discovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	EndSessionEndpoint    string   `json:"end_session_endpoint,omitempty"`
	ResponseTypes         []string `json:"response_types_supported"`
	SubjectTypes          []string `json:"subject_types_supported"`
	SigningAlgs           []string `json:"id_token_signing_alg_values_supported"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

type idClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Nonce    string `json:"nonce"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Username string `json:"preferred_username,omitempty"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

type tokenError struct {
	Error string `json:"error"`
}

// NewIssuer starts a provider that stops when t ends.
func NewIssuer(t testing.TB, opts Options) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	i := &Issuer{
		ClientID:     "kanban-test",
		ClientSecret: "kanban-secret",
		opts:         opts,
		key:          key,
		codes:        map[string]pending{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", i.discovery)
	mux.HandleFunc("GET /jwks", i.jwks)
	mux.HandleFunc("POST /token", i.token)
	i.server = httptest.NewServer(mux)
	i.URL = i.server.URL
	t.Cleanup(i.server.Close)
	return i
}

// EndSessionURL is the provider's end_session_endpoint.
func (i *Issuer) EndSessionURL() string { return i.URL + "/logout" }

// Authorize plays the reader's visit to the authorization endpoint: it checks
// the request the client built, and returns the path and query the browser is
// sent back to on the client.
func (i *Issuer) Authorize(t testing.TB, authURL string, g Grant) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("authtest: authorization URL: %v", err)
	}
	q := u.Query()
	want := map[string]string{
		"response_type":         "code",
		"client_id":             i.ClientID,
		"redirect_uri":          i.opts.RedirectURL,
		"code_challenge_method": "S256",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Fatalf("authtest: %s = %q, want %q", k, q.Get(k), v)
		}
	}
	for _, k := range []string{"state", "nonce", "code_challenge", "scope"} {
		if q.Get(k) == "" {
			t.Fatalf("authtest: authorization request has no %s", k)
		}
	}
	code := random(t)
	i.mu.Lock()
	i.codes[code] = pending{grant: g, nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	i.mu.Unlock()

	back, err := url.Parse(i.opts.RedirectURL)
	if err != nil {
		t.Fatal(err)
	}
	bq := url.Values{"code": {code}, "state": {q.Get("state")}}
	back.RawQuery = bq.Encode()
	return back.RequestURI()
}

func (i *Issuer) discovery(w http.ResponseWriter, _ *http.Request) {
	d := discovery{
		Issuer:                i.URL,
		AuthorizationEndpoint: i.URL + "/authorize",
		TokenEndpoint:         i.URL + "/token",
		JWKSURI:               i.URL + "/jwks",
		ResponseTypes:         []string{"code"},
		SubjectTypes:          []string{"public"},
		SigningAlgs:           []string{"RS256"},
		CodeChallengeMethods:  []string{"S256"},
	}
	if i.opts.EndSession {
		d.EndSessionEndpoint = i.EndSessionURL()
	}
	writeJSON(w, http.StatusOK, d)
}

func (i *Issuer) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &i.key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig",
	}}})
}

func (i *Issuer) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, tokenError{"invalid_request"})
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != i.ClientID || secret != i.ClientSecret {
		writeJSON(w, http.StatusUnauthorized, tokenError{"invalid_client"})
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("redirect_uri") != i.opts.RedirectURL {
		writeJSON(w, http.StatusBadRequest, tokenError{"invalid_grant"})
		return
	}

	i.mu.Lock()
	p, found := i.codes[r.PostForm.Get("code")]
	delete(i.codes, r.PostForm.Get("code")) // a code is good once
	i.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !found || base64.RawURLEncoding.EncodeToString(sum[:]) != p.challenge {
		writeJSON(w, http.StatusBadRequest, tokenError{"invalid_grant"})
		return
	}

	claims := idClaims{
		Issuer:   i.URL,
		Subject:  p.grant.Subject,
		Audience: i.ClientID,
		Expiry:   time.Now().Add(5 * time.Minute).Unix(),
		IssuedAt: time.Now().Unix(),
		Nonce:    p.nonce,
		Email:    p.grant.Email,
		Name:     p.grant.Name,
		Username: p.grant.PreferredUsername,
	}
	if p.grant.Nonce != "" {
		claims.Nonce = p.grant.Nonce
	}
	if p.grant.Audience != "" {
		claims.Audience = p.grant.Audience
	}
	idToken, err := i.sign(claims)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, tokenError{"server_error"})
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{AccessToken: "access", TokenType: "Bearer", ExpiresIn: 300, IDToken: idToken})
}

func (i *Issuer) sign(claims idClaims) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: i.key, KeyID: "test", Algorithm: "RS256"}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return obj.CompactSerialize()
}

func writeJSON[T discovery | jose.JSONWebKeySet | tokenResponse | tokenError](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func random(t testing.TB) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
