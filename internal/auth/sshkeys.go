package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHKey is the safe view of a stored public key.
type SSHKey struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	Created     string `json:"created"`
	LastUsed    string `json:"lastUsed"`
}

// AddSSHKey parses an authorized-keys line, computes its SHA256 fingerprint,
// and stores it for the user.
func (m *Manager) AddSSHKey(ctx context.Context, userID int64, name, pubkeyText string) (*SSHKey, error) {
	pubkeyText = strings.TrimSpace(pubkeyText)
	pk, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(pubkeyText))
	if err != nil {
		return nil, errors.New("could not parse public key (expected ssh-ed25519 / ssh-rsa …)")
	}
	if name == "" {
		name = comment
	}
	if name == "" {
		name = "key"
	}
	fp := ssh.FingerprintSHA256(pk)
	// Store the canonical authorized-key form.
	canonical := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk)))

	res, err := m.db.ExecContext(ctx,
		`INSERT INTO ssh_keys (user_id, name, fingerprint, public_key) VALUES (?,?,?,?)`,
		userID, name, fp, canonical)
	if err != nil {
		return nil, errors.New("that key is already registered")
	}
	id, _ := res.LastInsertId()
	return &SSHKey{ID: id, Name: name, Fingerprint: fp}, nil
}

func (m *Manager) ListSSHKeys(ctx context.Context, userID int64) ([]SSHKey, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, name, fingerprint, created_at, COALESCE(last_used_at,'')
		 FROM ssh_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SSHKey
	for rows.Next() {
		var k SSHKey
		if err := rows.Scan(&k.ID, &k.Name, &k.Fingerprint, &k.Created, &k.LastUsed); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (m *Manager) DeleteSSHKey(ctx context.Context, userID, id int64) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// UserBySSHKey resolves an offered public key to its owning user, by fingerprint.
func (m *Manager) UserBySSHKey(ctx context.Context, key ssh.PublicKey) (*User, error) {
	fp := ssh.FingerprintSHA256(key)
	var userID int64
	if err := m.db.QueryRowContext(ctx, `SELECT user_id FROM ssh_keys WHERE fingerprint = ?`, fp).Scan(&userID); err != nil {
		return nil, err
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = m.db.ExecContext(c, `UPDATE ssh_keys SET last_used_at = datetime('now') WHERE fingerprint = ?`, fp)
	}()
	return m.GetUser(ctx, userID)
}
