package auth

import (
	"context"
	"errors"
	"strings"
)

// Local email+password accounts. Password hashes reuse the same Argon2id
// helpers (hashToken/verifyToken) used for PATs — 64 MiB, t=3, p=2.

// ErrBadCredentials is returned for any login failure (wrong password, no such
// user, OIDC-only account, disabled account) — deliberately indistinct so the
// API can't be used to enumerate accounts.
var ErrBadCredentials = errors.New("invalid email or password")

const minPasswordLen = 8

// SetPassword sets (or clears, if password == "") a user's local password.
func (m *Manager) SetPassword(ctx context.Context, userID int64, password string) error {
	hash := ""
	if password != "" {
		if len(password) < minPasswordLen {
			return errors.New("password must be at least 8 characters")
		}
		hash = hashToken(password)
	}
	_, err := m.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

// CreateLocalUser provisions a new email+password account. The handle is derived
// from the email local-part when not supplied, made unique on collision. The
// very first user on a fresh instance becomes admin.
func (m *Manager) CreateLocalUser(ctx context.Context, email, handle, name, password string) (int64, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return 0, errors.New("a valid email is required")
	}
	if len(password) < minPasswordLen {
		return 0, errors.New("password must be at least 8 characters")
	}
	// Reject duplicate email (case-insensitive via the column's NOCASE collation).
	var existing int64
	if m.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&existing); existing != 0 {
		return 0, errors.New("an account with that email already exists")
	}
	if handle == "" {
		handle = strings.SplitN(email, "@", 2)[0]
	}
	uniqueHandle, err := m.uniqueHandle(ctx, sanitizeHandle(handle))
	if err != nil {
		return 0, err
	}
	if name == "" {
		name = uniqueHandle
	}
	res, err := m.db.ExecContext(ctx,
		`INSERT INTO users (handle, email, name, password_hash, last_login_at) VALUES (?,?,?,?, datetime('now'))`,
		uniqueHandle, email, name, hashToken(password))
	if err != nil {
		return 0, err
	}
	userID, _ := res.LastInsertId()
	// First user on the instance is the admin (bootstrap).
	var total int
	_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	if total == 1 {
		_, _ = m.db.ExecContext(ctx, `UPDATE users SET is_admin = 1 WHERE id = ?`, userID)
	}
	return userID, nil
}

// VerifyLogin checks an email-or-handle + password and returns the user. All
// failure modes collapse to ErrBadCredentials.
func (m *Manager) VerifyLogin(ctx context.Context, identifier, password string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return nil, ErrBadCredentials
	}
	var id int64
	var hash string
	var disabled int
	err := m.db.QueryRowContext(ctx,
		`SELECT id, password_hash, disabled FROM users WHERE email = ? OR handle = ?`,
		strings.ToLower(identifier), identifier).Scan(&id, &hash, &disabled)
	if err != nil || hash == "" || disabled == 1 {
		// Run a dummy verify to keep timing roughly constant for unknown users.
		_ = verifyToken(password, "argon2id$AAAA$AAAA")
		return nil, ErrBadCredentials
	}
	if !verifyToken(password, hash) {
		return nil, ErrBadCredentials
	}
	_, _ = m.db.ExecContext(ctx, `UPDATE users SET last_login_at = datetime('now') WHERE id = ?`, id)
	return m.GetUser(ctx, id)
}

// sanitizeHandle keeps handle derivation safe (alnum, dash, underscore, dot).
func sanitizeHandle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		out = "user"
	}
	return out
}
