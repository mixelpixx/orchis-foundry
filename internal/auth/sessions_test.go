package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/orchis-ai/foundry/internal/db"
)

func TestSessionListAndRevoke(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	res, err := d.ExecContext(ctx, `INSERT INTO users (handle) VALUES ('alice')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := res.LastInsertId()
	for _, id := range []string{"sessA", "sessB", "sessC"} {
		if _, err := d.ExecContext(ctx, `INSERT INTO sessions (id, user_id) VALUES (?,?)`, id, uid); err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}
	m := NewManager(d)

	// List: 3 sessions, exactly one flagged current; the public id must not be
	// the raw secret id.
	list, err := m.ListSessions(ctx, uid, "sessA")
	if err != nil || len(list) != 3 {
		t.Fatalf("list: err=%v n=%d", err, len(list))
	}
	cur := 0
	for _, si := range list {
		if si.Current {
			cur++
		}
		if si.PubID == "sessA" || si.PubID == "sessB" || si.PubID == "sessC" {
			t.Errorf("PubID leaked the raw session id: %q", si.PubID)
		}
	}
	if cur != 1 {
		t.Errorf("want exactly 1 current session, got %d", cur)
	}

	// Revoke sessB by its public handle → 2 remain.
	if err := m.RevokeSessionByPub(ctx, uid, sessionPub("sessB")); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if list, _ := m.ListSessions(ctx, uid, "sessA"); len(list) != 2 {
		t.Fatalf("after revoke want 2, got %d", len(list))
	}

	// Cannot revoke another user's session via our user id (no-op).
	res2, _ := d.ExecContext(ctx, `INSERT INTO users (handle) VALUES ('bob')`)
	bob, _ := res2.LastInsertId()
	d.ExecContext(ctx, `INSERT INTO sessions (id, user_id) VALUES ('sessBob', ?)`, bob)
	_ = m.RevokeSessionByPub(ctx, uid, sessionPub("sessBob"))
	var n int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id='sessBob'`).Scan(&n)
	if n != 1 {
		t.Errorf("revoked another user's session; cross-user isolation broken")
	}

	// Revoke others → only the current (sessA) remains.
	if err := m.RevokeOtherSessions(ctx, uid, "sessA"); err != nil {
		t.Fatalf("revoke others: %v", err)
	}
	list, _ = m.ListSessions(ctx, uid, "sessA")
	if len(list) != 1 || !list[0].Current {
		t.Fatalf("after revoke-others want 1 current session, got %d", len(list))
	}
}
