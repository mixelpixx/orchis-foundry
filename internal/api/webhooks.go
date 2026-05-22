package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// knownEvents mirrors the event chips in WebhooksPanel (src/views/devsettings.jsx).
var knownEvents = map[string]bool{
	"push": true, "pull_request": true, "issue": true,
	"release": true, "deploy": true, "comment": true,
}

// webhookJSON shapes a webhook row for the frontend (matches WEBHOOKS in
// src/data.jsx): { id, url, events[], repo, lastDelivery, status }.
func webhookJSON(id int64, url, events, repo string, lastAt sql.NullString, lastStatus sql.NullString) map[string]any {
	evs := []string{}
	for _, e := range strings.Split(events, ",") {
		if e = strings.TrimSpace(e); e != "" {
			evs = append(evs, e)
		}
	}
	last := "never"
	if lastAt.Valid && lastAt.String != "" {
		last = relativeTime(lastAt.String)
	}
	status := "ok"
	if lastStatus.Valid && lastStatus.String == "fail" {
		status = "warn"
	} else if !lastStatus.Valid || lastStatus.String == "" {
		status = "ok" // never delivered yet — show neutral/ok
	}
	return map[string]any{
		"id": strconv.FormatInt(id, 10), "url": url, "events": evs,
		"repo": repo, "lastDelivery": last, "status": status,
	}
}

// GET /v1/me/webhooks — flat list across all repos the user owns.
func (s *Server) handleListMyWebhooks(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT wh.id, wh.url, wh.events, u.handle, rp.name, wh.last_delivery_at, wh.last_delivery_status
		 FROM webhooks wh
		 JOIN repos rp ON rp.id = wh.repo_id
		 JOIN users u ON u.id = rp.owner_user_id
		 WHERE rp.owner_user_id = ?
		 ORDER BY wh.id DESC`, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list webhooks")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var url, events, owner, name string
		var lastAt, lastStatus sql.NullString
		if rows.Scan(&id, &url, &events, &owner, &name, &lastAt, &lastStatus) == nil {
			out = append(out, webhookJSON(id, url, events, owner+"/"+name, lastAt, lastStatus))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/{org}/{name}/webhooks
func (s *Server) handleListRepoWebhooks(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.ownsRepo(r, row) {
		writeError(w, http.StatusForbidden, "only the repo owner can manage webhooks")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT id, url, events, last_delivery_at, last_delivery_status FROM webhooks WHERE repo_id = ? ORDER BY id DESC`, row.ID)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var url, events string
			var lastAt, lastStatus sql.NullString
			if rows.Scan(&id, &url, &events, &lastAt, &lastStatus) == nil {
				out = append(out, webhookJSON(id, url, events, row.OwnerHandle+"/"+row.Name, lastAt, lastStatus))
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/repos/{org}/{name}/webhooks { url, events:[], secret? }
func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.ownsRepo(r, row) {
		writeError(w, http.StatusForbidden, "only the repo owner can manage webhooks")
		return
	}
	var in struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
		Secret string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		writeError(w, http.StatusBadRequest, "url must be http(s)")
		return
	}
	events := cleanEvents(in.Events)
	if events == "" {
		writeError(w, http.StatusBadRequest, "select at least one event")
		return
	}
	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO webhooks (repo_id, url, secret, events) VALUES (?,?,?,?)`,
		row.ID, in.URL, in.Secret, events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create webhook")
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusOK, webhookJSON(id, in.URL, events, row.OwnerHandle+"/"+row.Name, sql.NullString{}, sql.NullString{}))
}

// PATCH /v1/repos/{org}/{name}/webhooks/{id} { url?, events?, secret?, active? }
func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.ownsRepo(r, row) {
		writeError(w, http.StatusForbidden, "only the repo owner can manage webhooks")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !s.webhookBelongsToRepo(r, id, row.ID) {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	var in struct {
		URL    *string   `json:"url"`
		Events *[]string `json:"events"`
		Secret *string   `json:"secret"`
		Active *bool     `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.URL != nil {
		if !strings.HasPrefix(*in.URL, "http://") && !strings.HasPrefix(*in.URL, "https://") {
			writeError(w, http.StatusBadRequest, "url must be http(s)")
			return
		}
		s.db.ExecContext(r.Context(), `UPDATE webhooks SET url = ? WHERE id = ?`, *in.URL, id)
	}
	if in.Events != nil {
		events := cleanEvents(*in.Events)
		if events == "" {
			writeError(w, http.StatusBadRequest, "select at least one event")
			return
		}
		s.db.ExecContext(r.Context(), `UPDATE webhooks SET events = ? WHERE id = ?`, events, id)
	}
	if in.Secret != nil {
		s.db.ExecContext(r.Context(), `UPDATE webhooks SET secret = ? WHERE id = ?`, *in.Secret, id)
	}
	if in.Active != nil {
		s.db.ExecContext(r.Context(), `UPDATE webhooks SET active = ? WHERE id = ?`, boolToInt(*in.Active), id)
	}
	var url, events string
	var lastAt, lastStatus sql.NullString
	s.db.QueryRowContext(r.Context(),
		`SELECT url, events, last_delivery_at, last_delivery_status FROM webhooks WHERE id = ?`, id).
		Scan(&url, &events, &lastAt, &lastStatus)
	writeJSON(w, http.StatusOK, webhookJSON(id, url, events, row.OwnerHandle+"/"+row.Name, lastAt, lastStatus))
}

// DELETE /v1/repos/{org}/{name}/webhooks/{id}
func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.ownsRepo(r, row) {
		writeError(w, http.StatusForbidden, "only the repo owner can manage webhooks")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !s.webhookBelongsToRepo(r, id, row.ID) {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	s.db.ExecContext(r.Context(), `DELETE FROM webhooks WHERE id = ?`, id)
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/repos/{org}/{name}/webhooks/{id}/test — enqueue a sample ping.
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.ownsRepo(r, row) {
		writeError(w, http.StatusForbidden, "only the repo owner can manage webhooks")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !s.webhookBelongsToRepo(r, id, row.ID) {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if s.webhooks == nil {
		writeError(w, http.StatusServiceUnavailable, "delivery worker not running")
		return
	}
	// Enqueue a ping straight to this one webhook (bypasses event matching).
	s.webhooks.FireOne(r.Context(), id, "ping", map[string]any{
		"event": "ping", "repo": row.OwnerHandle + "/" + row.Name,
		"message": "This is a test delivery from Orchis Foundry.",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ownsRepo(r *http.Request, row *repoRow) bool {
	u := userFrom(r)
	return u != nil && row.OwnerUserID.Valid && row.OwnerUserID.Int64 == u.ID
}

func (s *Server) webhookBelongsToRepo(r *http.Request, id, repoID int64) bool {
	var n int
	s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM webhooks WHERE id = ? AND repo_id = ?`, id, repoID).Scan(&n)
	return n > 0
}

func cleanEvents(in []string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range in {
		e = strings.TrimSpace(e)
		if knownEvents[e] && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return strings.Join(out, ",")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
