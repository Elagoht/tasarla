// Package auth signs readers in with OpenID Connect and puts the signed-in
// user in the request's context.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"kanban/internal/store"
)

var (
	// ErrNonce reports an ID token that does not answer this sign-in.
	ErrNonce = errors.New("auth: id token nonce does not match")
	// ErrNoIDToken reports a token response with no ID token in it.
	ErrNoIDToken = errors.New("auth: token response has no id_token")
	// ErrNoEmail reports an ID token without the email claim.
	ErrNoEmail = errors.New("auth: id token has no email claim")
)

// ClientConfig names the provider and this application's registration with it.
type ClientConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Client is an OpenID Connect relying party using the authorization code flow
// with PKCE.
type Client struct {
	oauth      oauth2.Config
	verifier   *oidc.IDTokenVerifier
	clientID   string
	endSession string
}

type providerExtras struct {
	EndSessionEndpoint string `json:"end_session_endpoint"`
}

type profileClaims struct {
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

// NewClient runs discovery against the issuer; an unreachable or malformed
// provider is an error, so the application does not start without one.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovery at %s: %w", cfg.Issuer, err)
	}
	var extras providerExtras
	if err := provider.Claims(&extras); err != nil {
		return nil, fmt.Errorf("auth: discovery document: %w", err)
	}
	return &Client{
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
		},
		verifier:   provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		clientID:   cfg.ClientID,
		endSession: extras.EndSessionEndpoint,
	}, nil
}

// AuthURL is where the reader is sent to sign in.
func (c *Client) AuthURL(state, nonce, verifier string) string {
	return c.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

// Exchange trades the code for tokens and returns who the verified ID token
// says signed in. The token's issuer, audience and expiry are checked by the
// verifier; its nonce is checked here.
func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (store.Identity, error) {
	token, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return store.Identity{}, fmt.Errorf("auth: token exchange: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return store.Identity{}, ErrNoIDToken
	}
	idToken, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return store.Identity{}, fmt.Errorf("auth: id token: %w", err)
	}
	if nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return store.Identity{}, ErrNonce
	}
	var claims profileClaims
	if err := idToken.Claims(&claims); err != nil {
		return store.Identity{}, fmt.Errorf("auth: id token claims: %w", err)
	}
	if claims.Email == "" {
		return store.Identity{}, ErrNoEmail
	}
	// Authentik may send an empty name; its username is the next best thing,
	// and the store falls back to the email after that.
	name := claims.Name
	if strings.TrimSpace(name) == "" {
		name = claims.PreferredUsername
	}
	return store.Identity{Issuer: idToken.Issuer, Subject: idToken.Subject, Email: claims.Email, Name: name}, nil
}

// LogoutURL is the provider's RP-initiated logout for this client, returning
// the reader to postLogoutRedirect, or "" when the provider offers none.
func (c *Client) LogoutURL(postLogoutRedirect string) string {
	if c.endSession == "" {
		return ""
	}
	u, err := url.Parse(c.endSession)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", c.clientID)
	q.Set("post_logout_redirect_uri", postLogoutRedirect)
	u.RawQuery = q.Encode()
	return u.String()
}
