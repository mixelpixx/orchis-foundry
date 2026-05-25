// Package webhook delivers outbound webhook events to user-configured URLs.
//
// A single in-process worker drains the webhook_deliveries queue table, signs
// each payload with the webhook's HMAC-SHA256 secret, POSTs it, and records the
// result. Failed deliveries are retried with exponential backoff (1s, 4s, 16s,
// 1m, 5m, 30m) and given up after 6 attempts.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/orchis-ai/foundry/internal/netguard"
)

// Worker owns the delivery loop.
type Worker struct {
	db     *sql.DB
	log    *slog.Logger
	client *http.Client
	wake   chan struct{}
}

// backoff is the per-attempt delay before the Nth retry. After len(backoff)
// failed attempts the delivery is marked failed.
var backoff = []time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second, time.Minute, 5 * time.Minute, 30 * time.Minute}

func NewWorker(db *sql.DB, log *slog.Logger) *Worker {
	return &Worker{
		db:     db,
		log:    log,
		client: &http.Client{Timeout: 10 * time.Second},
		wake:   make(chan struct{}, 1),
	}
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Fire enqueues a delivery for every active webhook on repoID that subscribes
// to event. Best-effort; payload is marshalled to JSON. Safe to call from
// request handlers — it only inserts queue rows and nudges the worker.
func (w *Worker) Fire(ctx context.Context, repoID int64, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		w.log.Error("webhook payload marshal failed", "err", err)
		return
	}
	rows, err := w.db.QueryContext(ctx,
		`SELECT id, events FROM webhooks WHERE repo_id = ? AND active = 1`, repoID)
	if err != nil {
		return
	}
	type hook struct {
		id     int64
		events string
	}
	var hooks []hook
	for rows.Next() {
		var h hook
		if rows.Scan(&h.id, &h.events) == nil {
			hooks = append(hooks, h)
		}
	}
	rows.Close()

	queued := 0
	for _, h := range hooks {
		if !subscribed(h.events, event) {
			continue
		}
		_, err := w.db.ExecContext(ctx,
			`INSERT INTO webhook_deliveries (webhook_id, uuid, event, payload) VALUES (?,?,?,?)`,
			h.id, newUUID(), event, string(body))
		if err == nil {
			queued++
		}
	}
	if queued > 0 {
		w.nudge()
	}
}

// FireOne enqueues a single delivery to a specific webhook regardless of its
// event subscriptions (used by the "Test" button to send a ping).
func (w *Worker) FireOne(ctx context.Context, webhookID int64, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, err = w.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (webhook_id, uuid, event, payload) VALUES (?,?,?,?)`,
		webhookID, newUUID(), event, string(body))
	if err == nil {
		w.nudge()
	}
}

func subscribed(events, event string) bool {
	for _, e := range strings.Split(events, ",") {
		if strings.TrimSpace(e) == event {
			return true
		}
	}
	return false
}

func (w *Worker) nudge() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run loops until ctx is cancelled, delivering due jobs.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second) // safety poll for backed-off retries
	defer ticker.Stop()
	for {
		w.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

func (w *Worker) drain(ctx context.Context) {
	for {
		var id, webhookID, attempts int64
		var uuid, event, payload string
		err := w.db.QueryRowContext(ctx,
			`SELECT id, webhook_id, uuid, event, payload, attempts
			 FROM webhook_deliveries
			 WHERE status = 'queued' AND next_attempt_at <= datetime('now')
			 ORDER BY id LIMIT 1`).
			Scan(&id, &webhookID, &uuid, &event, &payload, &attempts)
		if err == sql.ErrNoRows {
			return
		}
		if err != nil {
			w.log.Error("webhook poll failed", "err", err)
			return
		}
		w.deliver(ctx, id, webhookID, uuid, event, payload, int(attempts))
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (w *Worker) deliver(ctx context.Context, id, webhookID int64, uuid, event, payload string, attempts int) {
	var url, secret string
	if err := w.db.QueryRowContext(ctx, `SELECT url, secret FROM webhooks WHERE id = ?`, webhookID).Scan(&url, &secret); err != nil {
		// Webhook deleted out from under the queue; drop the delivery.
		w.db.ExecContext(ctx, `UPDATE webhook_deliveries SET status='failed', error='webhook removed' WHERE id=?`, id)
		return
	}

	code, derr := w.post(ctx, url, secret, uuid, event, payload)
	attempts++
	now := time.Now()

	if derr == nil && code >= 200 && code < 300 {
		w.db.ExecContext(ctx,
			`UPDATE webhook_deliveries SET status='delivered', attempts=?, response_code=?, error=NULL, delivered_at=datetime('now') WHERE id=?`,
			attempts, code, id)
		w.db.ExecContext(ctx,
			`UPDATE webhooks SET last_delivery_at=datetime('now'), last_delivery_status='ok' WHERE id=?`, webhookID)
		w.log.Info("webhook delivered", "webhook", webhookID, "event", event, "code", code)
		return
	}

	errMsg := fmt.Sprintf("HTTP %d", code)
	if derr != nil {
		errMsg = derr.Error()
	}
	if attempts >= len(backoff) {
		w.db.ExecContext(ctx,
			`UPDATE webhook_deliveries SET status='failed', attempts=?, response_code=?, error=? WHERE id=?`,
			attempts, code, errMsg, id)
		w.db.ExecContext(ctx,
			`UPDATE webhooks SET last_delivery_at=datetime('now'), last_delivery_status='fail' WHERE id=?`, webhookID)
		w.log.Warn("webhook gave up", "webhook", webhookID, "event", event, "err", errMsg)
		return
	}
	next := now.Add(backoff[attempts-1]).UTC().Format("2006-01-02 15:04:05")
	w.db.ExecContext(ctx,
		`UPDATE webhook_deliveries SET attempts=?, response_code=?, error=?, next_attempt_at=? WHERE id=?`,
		attempts, code, errMsg, next, id)
	w.db.ExecContext(ctx,
		`UPDATE webhooks SET last_delivery_at=datetime('now'), last_delivery_status='fail' WHERE id=?`, webhookID)
	w.log.Info("webhook retry scheduled", "webhook", webhookID, "attempt", attempts, "next", next)
}

func (w *Worker) post(ctx context.Context, url, secret, uuid, event, payload string) (int, error) {
	// Re-validate at delivery time (defense against DNS rebinding or a target
	// that became internal after it was saved).
	if err := netguard.ValidateOutboundURL(url); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Orchis-Foundry-Webhook/1")
	req.Header.Set("X-Orchis-Event", event)
	req.Header.Set("X-Orchis-Delivery", uuid)
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		req.Header.Set("X-Orchis-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
