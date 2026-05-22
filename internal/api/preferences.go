package api

import (
	"encoding/json"
	"net/http"
)

// allowedPrefs is a strict allowlist of preference keys → permitted values.
// A nil value list means "any boolean". Anything not listed is ignored, so a
// client can't stuff arbitrary data into the user row.
var allowedPrefs = map[string]map[string]bool{
	"theme":   {"light": true, "dark": true},
	"accent":  {"spring": true, "cobalt": true, "ember": true, "violet": true},
	"density": {"default": true, "compact": true, "cozy": true},
	"font":    {"geist": true, "serifMix": true, "mono": true},
}
var boolPrefs = map[string]bool{"showSplitTip": true}

// loadPrefs returns the user's stored preferences as a map (empty if unset).
func (s *Server) loadPrefs(r *http.Request, userID int64) map[string]any {
	var raw string
	s.db.QueryRowContext(r.Context(), `SELECT prefs FROM users WHERE id = ?`, userID).Scan(&raw)
	m := map[string]any{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	return m
}

// GET /v1/me/preferences
func (s *Server) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	writeJSON(w, http.StatusOK, s.loadPrefs(r, u.ID))
}

// PATCH /v1/me/preferences { theme?, accent?, density?, font?, showSplitTip? }
// Merges validated keys into the stored preferences.
func (s *Server) handlePatchPreferences(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var in map[string]any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	cur := s.loadPrefs(r, u.ID)
	for k, v := range in {
		if vals, ok := allowedPrefs[k]; ok {
			if sv, isStr := v.(string); isStr && vals[sv] {
				cur[k] = sv
			}
			continue
		}
		if boolPrefs[k] {
			if bv, isBool := v.(bool); isBool {
				cur[k] = bv
			}
		}
	}
	out, _ := json.Marshal(cur)
	if _, err := s.db.ExecContext(r.Context(), `UPDATE users SET prefs = ? WHERE id = ?`, string(out), u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save preferences")
		return
	}
	writeJSON(w, http.StatusOK, cur)
}
