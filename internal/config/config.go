// Package config loads Orchis Foundry configuration from a YAML file with
// environment-variable overrides. Intentionally tiny — no viper, no magic.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config is the full runtime configuration.
type Config struct {
	HTTPAddr    string // e.g. "127.0.0.1:8090"
	SSHAddr     string // e.g. ":2222"
	ExternalURL string // e.g. "https://foundry.orchis.ai"

	DBPath    string // sqlite file, e.g. /var/lib/orchis-foundry/foundry.db
	ReposDir  string // bare repos root
	WebDir    string // optional override for the embedded web/ assets ("" = use embedded)
	SSHHostKey string // path to the SSH server host private key

	SessionKey   string // base64, 32 bytes, for PASETO local tokens
	AnthropicKey string // for the LLM supply-chain scanner

	OIDC []OIDCProvider
}

// OIDCProvider is one configured identity provider.
type OIDCProvider struct {
	ID           string // "github", "google", "generic"
	Issuer       string // OIDC issuer URL
	AuthURL      string // override (GitHub isn't a compliant OIDC issuer)
	TokenURL     string // override
	UserInfoURL  string // override
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Load reads the YAML config at path, then applies env overrides.
//
// We parse a deliberately small subset of YAML by hand to avoid a dependency;
// the file format is flat "key: value" plus an [oidc] repeated block. To keep
// things robust we actually accept a minimal indentation-based format handled
// in parseYAML below.
func Load(path string) (*Config, error) {
	c := &Config{
		HTTPAddr:   "127.0.0.1:8090",
		SSHAddr:    ":2222",
		DBPath:     "/var/lib/orchis-foundry/foundry.db",
		ReposDir:   "/var/lib/orchis-foundry/repos",
		SSHHostKey: "/var/lib/orchis-foundry/ssh_host_ed25519_key",
	}

	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
		if err := parseYAML(string(raw), c); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// Env overrides (used by the systemd unit / dev shell).
	envOverride("ORCHIS_HTTP_ADDR", &c.HTTPAddr)
	envOverride("ORCHIS_SSH_ADDR", &c.SSHAddr)
	envOverride("ORCHIS_EXTERNAL_URL", &c.ExternalURL)
	envOverride("ORCHIS_DB_PATH", &c.DBPath)
	envOverride("ORCHIS_REPOS_DIR", &c.ReposDir)
	envOverride("ORCHIS_WEB_DIR", &c.WebDir)
	envOverride("ORCHIS_SSH_HOST_KEY", &c.SSHHostKey)
	envOverride("ORCHIS_SESSION_KEY", &c.SessionKey)
	envOverride("ANTHROPIC_API_KEY", &c.AnthropicKey)

	if c.ExternalURL == "" {
		c.ExternalURL = "http://" + c.HTTPAddr
	}
	return c, nil
}

func envOverride(key string, dst *string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

// parseYAML handles our small config shape. Supported:
//
//	server:
//	  http_addr: "127.0.0.1:8090"
//	  ssh_addr: ":2222"
//	  external_url: "https://foundry.orchis.ai"
//	storage:
//	  db_path: "/var/lib/orchis-foundry/foundry.db"
//	  repos_dir: "/var/lib/orchis-foundry/repos"
//	  ssh_host_key: "/var/lib/orchis-foundry/ssh_host_ed25519_key"
//	auth:
//	  session_key: "base64..."
//	scan:
//	  anthropic_api_key: "sk-ant-..."
//	oidc:
//	  - id: github
//	    client_id: "..."
//	    client_secret: "..."
//	  - id: google
//	    client_id: "..."
//	    client_secret: "..."
func parseYAML(src string, c *Config) error {
	var section string
	var curOIDC *OIDCProvider
	flush := func() {
		if curOIDC != nil {
			c.OIDC = append(c.OIDC, *curOIDC)
			curOIDC = nil
		}
	}
	for _, rawLine := range strings.Split(src, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)

		// Top-level section header (no indent, ends with ':').
		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			flush()
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		// OIDC list item start.
		if section == "oidc" && strings.HasPrefix(trimmed, "- ") {
			flush()
			curOIDC = &OIDCProvider{}
			trimmed = strings.TrimPrefix(trimmed, "- ")
		}

		key, val, ok := splitKV(trimmed)
		if !ok {
			continue
		}
		val = unquote(val)

		switch section {
		case "server":
			switch key {
			case "http_addr":
				c.HTTPAddr = val
			case "ssh_addr":
				c.SSHAddr = val
			case "external_url":
				c.ExternalURL = val
			}
		case "storage":
			switch key {
			case "db_path":
				c.DBPath = val
			case "repos_dir":
				c.ReposDir = val
			case "web_dir":
				c.WebDir = val
			case "ssh_host_key":
				c.SSHHostKey = val
			}
		case "auth":
			if key == "session_key" {
				c.SessionKey = val
			}
		case "scan":
			if key == "anthropic_api_key" {
				c.AnthropicKey = val
			}
		case "oidc":
			if curOIDC == nil {
				curOIDC = &OIDCProvider{}
			}
			switch key {
			case "id":
				curOIDC.ID = val
			case "issuer":
				curOIDC.Issuer = val
			case "auth_url":
				curOIDC.AuthURL = val
			case "token_url":
				curOIDC.TokenURL = val
			case "userinfo_url":
				curOIDC.UserInfoURL = val
			case "client_id":
				curOIDC.ClientID = val
			case "client_secret":
				curOIDC.ClientSecret = val
			case "scopes":
				curOIDC.Scopes = splitList(val)
			}
		}
	}
	flush()
	return nil
}

func splitKV(s string) (string, string, bool) {
	i := strings.Index(s, ":")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

func splitList(s string) []string {
	s = strings.Trim(s, "[]")
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(unquote(strings.TrimSpace(p))); p != "" {
			out = append(out, p)
		}
	}
	return out
}
