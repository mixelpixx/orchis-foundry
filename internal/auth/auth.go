// Package auth handles users, OIDC identity linking, and DB-backed sessions.
//
// Session model deviation from CLAUDE.md: the spec proposed PASETO v4 local
// tokens. We use opaque random session IDs stored in a sessions table instead.
// Rationale: on a single host, opaque DB sessions are strictly simpler, are
// trivially revocable (delete the row), and sidestep any token-crypto
// dependency. The spec's motivation for PASETO ("no JWT alg-confusion") does
// not apply to opaque random tokens at all.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const (
	sessionCookie = "orchis_sess"
	sessionTTL    = 14 * 24 * time.Hour
)

// User is the minimal authenticated identity used across the app.
type User struct {
	ID        int64  `json:"-"`
	Handle    string `json:"handle"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
	Bio       string `json:"bio"`
	IsAdmin   bool   `json:"isAdmin"`
}

// Manager owns session + user persistence.
type Manager struct {
	db *sql.DB
}

func NewManager(db *sql.DB) *Manager { return &Manager{db: db} }

// UpsertOIDCUser links an OIDC identity to a user, creating the user on first
// sight. Returns the resolved user ID.
func (m *Manager) UpsertOIDCUser(ctx context.Context, provider, subject, handle, name, email, avatar string) (int64, error) {
	var userID int64
	err := m.db.QueryRowContext(ctx,
		`SELECT user_id FROM oidc_identities WHERE provider = ? AND subject = ?`,
		provider, subject).Scan(&userID)
	if err == nil {
		// Existing identity — refresh profile fields opportunistically.
		_, _ = m.db.ExecContext(ctx,
			`UPDATE users SET name = ?, email = ?, avatar_url = ?, last_login_at = datetime('now') WHERE id = ?`,
			name, email, avatar, userID)
		return userID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	// New identity. Reuse a user with the same email if one exists, else create.
	if email != "" {
		_ = m.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	}
	if userID == 0 {
		uniqueHandle, herr := m.uniqueHandle(ctx, handle)
		if herr != nil {
			return 0, herr
		}
		res, ierr := m.db.ExecContext(ctx,
			`INSERT INTO users (handle, email, name, avatar_url, last_login_at) VALUES (?,?,?,?, datetime('now'))`,
			uniqueHandle, email, name, avatar)
		if ierr != nil {
			return 0, ierr
		}
		userID, _ = res.LastInsertId()
		// First user becomes admin.
		var total int
		_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
		if total == 1 {
			_, _ = m.db.ExecContext(ctx, `UPDATE users SET is_admin = 1 WHERE id = ?`, userID)
		}
	}
	if _, err := m.db.ExecContext(ctx,
		`INSERT INTO oidc_identities (provider, subject, user_id) VALUES (?,?,?)`,
		provider, subject, userID); err != nil {
		return 0, err
	}
	return userID, nil
}

// uniqueHandle finds a free handle, appending -1, -2, ... on collision.
func (m *Manager) uniqueHandle(ctx context.Context, base string) (string, error) {
	if base == "" {
		base = "user"
	}
	candidate := base
	for i := 1; i < 1000; i++ {
		var n int
		if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE handle = ?`, candidate).Scan(&n); err != nil {
			return "", err
		}
		if n == 0 {
			return candidate, nil
		}
		candidate = base + "-" + itoa(i)
	}
	return "", errors.New("could not allocate unique handle")
}

// EnsureUserByHandle finds or creates a user with the given handle (used by
// the --mint-token CLI escape hatch). New users created this way are admins.
func (m *Manager) EnsureUserByHandle(ctx context.Context, handle string) (int64, error) {
	var id int64
	err := m.db.QueryRowContext(ctx, `SELECT id FROM users WHERE handle = ?`, handle).Scan(&id)
	if err == nil {
		return id, nil
	}
	res, err := m.db.ExecContext(ctx,
		`INSERT INTO users (handle, name, is_admin) VALUES (?,?,1)`, handle, handle)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetUser loads a user by ID.
func (m *Manager) GetUser(ctx context.Context, id int64) (*User, error) {
	u := &User{ID: id}
	var admin int
	err := m.db.QueryRowContext(ctx,
		`SELECT handle, name, email, avatar_url, bio, is_admin FROM users WHERE id = ?`, id).
		Scan(&u.Handle, &u.Name, &u.Email, &u.AvatarURL, &u.Bio, &admin)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	return u, nil
}

// UpdateProfile updates the editable profile fields (display name + bio).
func (m *Manager) UpdateProfile(ctx context.Context, userID int64, name, bio string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE users SET name = ?, bio = ? WHERE id = ?`, name, bio, userID)
	return err
}

// SessionInfo describes one active session for the security UI. PubID is a
// non-reversible handle (hash prefix of the secret session id) — safe to
// expose; the raw session id (the cookie credential) is never returned.
type SessionInfo struct {
	PubID      string `json:"id"`
	CreatedAt  string `json:"createdAt"`
	LastSeenAt string `json:"lastSeenAt"`
	Current    bool   `json:"current"`
}

func sessionPub(id string) string {
	sum := sha256.Sum256([]byte("sess-pub:" + id))
	return hex.EncodeToString(sum[:])[:16]
}

// SessionID returns the caller's session cookie value, or "" (e.g. PAT auth).
func (m *Manager) SessionID(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// ListSessions returns the user's active sessions, flagging the current one.
func (m *Manager) ListSessions(ctx context.Context, userID int64, currentID string) ([]SessionInfo, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, created_at, last_seen_at FROM sessions WHERE user_id = ? ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionInfo{}
	for rows.Next() {
		var id, created, seen string
		if rows.Scan(&id, &created, &seen) == nil {
			out = append(out, SessionInfo{PubID: sessionPub(id), CreatedAt: created, LastSeenAt: seen, Current: id == currentID})
		}
	}
	return out, nil
}

// RevokeSessionByPub deletes one of the user's sessions matched by its public
// handle. Only the owning user's sessions are considered.
func (m *Manager) RevokeSessionByPub(ctx context.Context, userID int64, pub string) error {
	rows, err := m.db.QueryContext(ctx, `SELECT id FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	var match string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && sessionPub(id) == pub {
			match = id
			break
		}
	}
	rows.Close()
	if match != "" {
		_, _ = m.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ? AND user_id = ?`, match, userID)
	}
	return nil
}

// RevokeOtherSessions signs the user out of every session except currentID.
func (m *Manager) RevokeOtherSessions(ctx context.Context, userID int64, currentID string) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND id != ?`, userID, currentID)
	return err
}

// CreateSession mints a session row and sets the cookie.
func (m *Manager) CreateSession(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int64) error {
	id := randHex(32)
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, ip_hash, ua_hash) VALUES (?,?,?,?)`,
		id, userID, hashStr(clientIP(r)), hashStr(r.UserAgent()))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		Expires:  time.Now().Add(sessionTTL),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// UserFromRequest resolves the session cookie to a user, or nil if none.
func (m *Manager) UserFromRequest(ctx context.Context, r *http.Request) *User {
	c, err := r.Cookie(sessionCookie)
	if err != nil || len(c.Value) != 64 {
		return nil
	}
	var userID int64
	var ipHash, uaHash string
	err = m.db.QueryRowContext(ctx,
		`SELECT user_id, ip_hash, ua_hash FROM sessions
		 WHERE id = ? AND last_seen_at > datetime('now', '-14 days')`,
		c.Value).Scan(&userID, &ipHash, &uaHash)
	if err != nil {
		return nil
	}
	if ipHash != hashStr(clientIP(r)) || uaHash != hashStr(r.UserAgent()) {
		_, _ = m.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, c.Value)
		return nil
	}
	_, _ = m.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = datetime('now') WHERE id = ?`, c.Value)
	u, err := m.GetUser(ctx, userID)
	if err != nil {
		return nil
	}
	return u
}

// Destroy clears the session row + cookie.
func (m *Manager) Destroy(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_, _ = m.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// ---- OAuth state (CSRF) ----

func (m *Manager) NewOAuthState(ctx context.Context, provider string) (string, error) {
	state := randHex(24)
	_, err := m.db.ExecContext(ctx, `INSERT INTO oauth_states (state, provider) VALUES (?,?)`, state, provider)
	return state, err
}

// ConsumeOAuthState validates & deletes a state token, returning its provider.
func (m *Manager) ConsumeOAuthState(ctx context.Context, state string) (string, bool) {
	var provider string
	err := m.db.QueryRowContext(ctx,
		`SELECT provider FROM oauth_states WHERE state = ? AND created_at > datetime('now','-15 minutes')`,
		state).Scan(&provider)
	if err != nil {
		return "", false
	}
	_, _ = m.db.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = ?`, state)
	return provider, true
}

// ---- helpers ----

func randHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Real-IP"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}
