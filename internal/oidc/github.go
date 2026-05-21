// Package oidc implements provider sign-in. GitHub is plain OAuth2 (not a
// compliant OIDC issuer), so we use its OAuth2 endpoints + REST user API.
// Google (a real OIDC provider) can be added alongside using discovery later.
package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Profile is the normalized identity returned from a provider.
type Profile struct {
	Provider string
	Subject  string // stable provider-side user id
	Handle   string
	Name     string
	Email    string
	Avatar   string
}

// GitHub holds the OAuth2 client config for github.com.
type GitHub struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	http         *http.Client
}

func NewGitHub(clientID, clientSecret, redirectURI string) *GitHub {
	return &GitHub{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		http:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *GitHub) Configured() bool { return g.ClientID != "" && g.ClientSecret != "" }

// AuthURL builds the authorize redirect for the given state.
func (g *GitHub) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", g.RedirectURI)
	q.Set("scope", "read:user user:email")
	q.Set("state", state)
	q.Set("allow_signup", "false")
	return "https://github.com/login/oauth/authorize?" + q.Encode()
}

// Exchange swaps an authorization code for a normalized Profile.
func (g *GitHub) Exchange(ctx context.Context, code string) (*Profile, error) {
	token, err := g.exchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return g.fetchProfile(ctx, token)
}

func (g *GitHub) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", g.RedirectURI)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("github oauth: %s: %s", out.Error, out.ErrorDesc)
	}
	if out.AccessToken == "" {
		return "", errors.New("github oauth: empty access token")
	}
	return out.AccessToken, nil
}

func (g *GitHub) fetchProfile(ctx context.Context, token string) (*Profile, error) {
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := g.apiGet(ctx, token, "https://api.github.com/user", &u); err != nil {
		return nil, err
	}

	email := u.Email
	if email == "" {
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := g.apiGet(ctx, token, "https://api.github.com/user/emails", &emails); err == nil {
			for _, e := range emails {
				if e.Primary && e.Verified {
					email = e.Email
					break
				}
			}
		}
	}

	name := u.Name
	if name == "" {
		name = u.Login
	}
	return &Profile{
		Provider: "github",
		Subject:  strconv.FormatInt(u.ID, 10),
		Handle:   u.Login,
		Name:     name,
		Email:    email,
		Avatar:   u.AvatarURL,
	}, nil
}

func (g *GitHub) apiGet(ctx context.Context, token, urlStr string, dst any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "orchis-foundry")
	resp, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api %s: status %d", urlStr, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
