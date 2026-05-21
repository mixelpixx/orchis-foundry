package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// PAT format: orc_pat_<22 base62 chars>. The 8 chars immediately after the
// fixed "orc_pat_" prefix are stored separately as a lookup peek so we can
// narrow the candidate set to ~1 row before running argon2 verification.
const (
	patPrefix    = "orc_pat_"
	patRandomLen = 22 // base62 chars
	patPeekLen   = 8
)

// argon2id parameters (CLAUDE.md §10): 64 MiB, t=3, p=2.
const (
	argonMem  = 64 * 1024
	argonTime = 3
	argonPar  = 2
	argonKey  = 32
	argonSalt = 16
)

// TokenCreated is returned exactly once when a PAT is minted.
type TokenCreated struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Token     string   `json:"token"` // plaintext — shown ONCE
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires,omitempty"`
}

// TokenInfo is the safe (no-secret) view of a PAT.
type TokenInfo struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
	Created  string   `json:"created"`
	LastUsed string   `json:"lastUsed"`
	Expires  string   `json:"expires"`
	Expired  bool     `json:"expired"`
}

const base62 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func randBase62(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = base62[int(b[i])%len(base62)]
	}
	return string(b)
}

// CreateToken mints a PAT for the user, storing only the argon2id hash.
func (m *Manager) CreateToken(ctx context.Context, userID int64, name string, scopes []string, expiresInDays int) (*TokenCreated, error) {
	if name == "" {
		name = "token"
	}
	secret := randBase62(patRandomLen)
	token := patPrefix + secret
	peek := secret[:patPeekLen]
	hash := hashToken(token)

	var expiresAt any
	var expiresStr string
	if expiresInDays > 0 {
		exp := time.Now().Add(time.Duration(expiresInDays) * 24 * time.Hour).UTC()
		expiresStr = exp.Format("2006-01-02")
		expiresAt = exp.Format("2006-01-02 15:04:05")
	}

	res, err := m.db.ExecContext(ctx,
		`INSERT INTO personal_access_tokens (user_id, name, hash, prefix, scopes, expires_at)
		 VALUES (?,?,?,?,?,?)`,
		userID, name, hash, peek, strings.Join(scopes, ","), expiresAt)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &TokenCreated{ID: id, Name: name, Token: token, Scopes: scopes, ExpiresAt: expiresStr}, nil
}

// ListTokens returns the user's tokens without secrets.
func (m *Manager) ListTokens(ctx context.Context, userID int64) ([]TokenInfo, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, name, scopes, created_at, COALESCE(last_used_at,''), COALESCE(expires_at,'')
		 FROM personal_access_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TokenInfo
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for rows.Next() {
		var t TokenInfo
		var scopes string
		if err := rows.Scan(&t.ID, &t.Name, &scopes, &t.Created, &t.LastUsed, &t.Expires); err != nil {
			return nil, err
		}
		if scopes != "" {
			t.Scopes = strings.Split(scopes, ",")
		}
		t.Expired = t.Expires != "" && t.Expires < now
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteToken revokes a token (must belong to the user).
func (m *Manager) DeleteToken(ctx context.Context, userID, tokenID int64) error {
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM personal_access_tokens WHERE id = ? AND user_id = ?`, tokenID, userID)
	return err
}

// TokenAuth holds the result of validating a PAT.
type TokenAuth struct {
	User   *User
	Scopes []string
}

// ValidateToken parses & verifies a PAT, returning the owning user + scopes.
func (m *Manager) ValidateToken(ctx context.Context, token string) (*TokenAuth, error) {
	if !strings.HasPrefix(token, patPrefix) {
		return nil, errors.New("bad token prefix")
	}
	secret := strings.TrimPrefix(token, patPrefix)
	if len(secret) < patPeekLen {
		return nil, errors.New("bad token length")
	}
	peek := secret[:patPeekLen]

	rows, err := m.db.QueryContext(ctx,
		`SELECT id, user_id, hash, scopes, COALESCE(expires_at,'') FROM personal_access_tokens WHERE prefix = ?`, peek)
	if err != nil {
		return nil, err
	}

	// Drain the cursor fully (and close it) BEFORE any nested query. With a
	// single DB connection, calling GetUser/ExecContext while this cursor is
	// open would deadlock waiting for the connection the cursor holds.
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	var matchedID, matchedUser int64
	var matchedScopes string
	found := false
	for rows.Next() {
		var id, userID int64
		var hash, scopes, expires string
		if err := rows.Scan(&id, &userID, &hash, &scopes, &expires); err != nil {
			rows.Close()
			return nil, err
		}
		if expires != "" && expires < now {
			continue
		}
		if verifyToken(token, hash) {
			matchedID, matchedUser, matchedScopes = id, userID, scopes
			found = true
			break
		}
	}
	rows.Close()
	if !found {
		return nil, errors.New("invalid token")
	}

	// Update last_used_at asynchronously (own connection turn).
	go func(tid int64) {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = m.db.ExecContext(c, `UPDATE personal_access_tokens SET last_used_at = datetime('now') WHERE id = ?`, tid)
	}(matchedID)

	u, err := m.GetUser(ctx, matchedUser)
	if err != nil {
		return nil, err
	}
	var sc []string
	if matchedScopes != "" {
		sc = strings.Split(matchedScopes, ",")
	}
	return &TokenAuth{User: u, Scopes: sc}, nil
}

// hashToken returns an encoded argon2id hash: argon2id$<b64salt>$<b64key>.
func hashToken(token string) string {
	salt := make([]byte, argonSalt)
	_, _ = rand.Read(salt)
	key := argon2.IDKey([]byte(token), salt, argonTime, argonMem, argonPar, argonKey)
	return "argon2id$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}

func verifyToken(token, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(token), salt, argonTime, argonMem, argonPar, argonKey)
	return subtle.ConstantTimeCompare(got, want) == 1
}
