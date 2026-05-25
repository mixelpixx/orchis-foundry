package api

import "context"

// Instance settings (key/value). Currently used for admin-controlled auth
// toggles; deliberately generic so other instance config can ride along.
//
// Keys in use:
//   auth.local_login     "on"|"off"  (default on)
//   auth.local_signup    "on"|"off"  (default off)
//   auth.provider.<id>   "on"|"off"  (default on) — enable a configured OIDC provider

func (s *Server) getSetting(ctx context.Context, key string) (string, bool) {
	var v string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v); err != nil {
		return "", false
	}
	return v, true
}

// settingBool returns the boolean setting, or def when unset.
func (s *Server) settingBool(ctx context.Context, key string, def bool) bool {
	v, ok := s.getSetting(ctx, key)
	if !ok {
		return def
	}
	return v == "on" || v == "true" || v == "1"
}

func (s *Server) setSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?,?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		key, value)
	return err
}

// authSettings resolves the effective auth configuration: which methods are
// enabled and which configured OIDC providers are live.
type authMethod struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (s *Server) enabledProviders(ctx context.Context) []authMethod {
	out := []authMethod{}
	for _, p := range s.cfg.OIDC {
		if p.ID == "" || p.ClientID == "" {
			continue // not actually configured
		}
		if !s.settingBool(ctx, "auth.provider."+p.ID, true) {
			continue // admin disabled it
		}
		out = append(out, authMethod{ID: p.ID, Label: providerLabel(p.ID)})
	}
	return out
}

func providerLabel(id string) string {
	switch id {
	case "github":
		return "GitHub"
	case "google":
		return "Google"
	case "entra", "azure", "microsoft":
		return "Microsoft"
	case "generic", "oidc", "sso":
		return "SSO"
	case "":
		return "SSO"
	default:
		return id[:1] + id[1:]
	}
}
