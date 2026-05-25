package oidc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Generic is a standards-compliant OIDC provider (Microsoft Entra ID, Google,
// Okta, Keycloak, Auth0, …) configured by its issuer URL. Discovery and ID-token
// signature verification (JWKS) are handled by go-oidc. Initialization is lazy
// so a momentarily-unreachable IdP doesn't block server startup.
type Generic struct {
	id           string
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	scopes       []string

	mu       sync.Mutex
	provider *gooidc.Provider
	verifier *gooidc.IDTokenVerifier
	oauth    *oauth2.Config
}

// NewGeneric builds a generic OIDC provider. scopes defaults to openid/profile/email.
func NewGeneric(id, issuer, clientID, clientSecret, redirectURI string, scopes []string) *Generic {
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "profile", "email"}
	}
	return &Generic{
		id: id, issuer: issuer, clientID: clientID, clientSecret: clientSecret,
		redirectURI: redirectURI, scopes: scopes,
	}
}

func (g *Generic) Configured() bool {
	return g.issuer != "" && g.clientID != "" && g.clientSecret != ""
}

// ensure performs (once) OIDC discovery against the issuer and builds the
// oauth2 config + ID-token verifier.
func (g *Generic) ensure(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.provider != nil {
		return nil
	}
	p, err := gooidc.NewProvider(ctx, g.issuer)
	if err != nil {
		return fmt.Errorf("oidc discovery (%s): %w", g.issuer, err)
	}
	g.provider = p
	g.verifier = p.Verifier(&gooidc.Config{ClientID: g.clientID})
	g.oauth = &oauth2.Config{
		ClientID:     g.clientID,
		ClientSecret: g.clientSecret,
		RedirectURL:  g.redirectURI,
		Endpoint:     p.Endpoint(),
		Scopes:       g.scopes,
	}
	return nil
}

// AuthURL builds the authorize redirect for the given CSRF state.
func (g *Generic) AuthURL(ctx context.Context, state string) (string, error) {
	if err := g.ensure(ctx); err != nil {
		return "", err
	}
	return g.oauth.AuthCodeURL(state), nil
}

// Exchange swaps an auth code for a verified ID token and a normalized Profile.
func (g *Generic) Exchange(ctx context.Context, code string) (*Profile, error) {
	if err := g.ensure(ctx); err != nil {
		return nil, err
	}
	tok, err := g.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, errors.New("no id_token in provider response")
	}
	idTok, err := g.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("id token verification failed: %w", err)
	}
	var c struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Picture           string `json:"picture"`
	}
	if err := idTok.Claims(&c); err != nil {
		return nil, err
	}
	handle := c.PreferredUsername
	if handle == "" && c.Email != "" {
		handle = strings.SplitN(c.Email, "@", 2)[0]
	}
	if handle == "" {
		handle = c.Sub
	}
	name := c.Name
	if name == "" {
		name = handle
	}
	return &Profile{
		Provider: g.id, Subject: c.Sub, Handle: handle,
		Name: name, Email: c.Email, Avatar: c.Picture,
	}, nil
}
