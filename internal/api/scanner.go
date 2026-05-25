package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/orchis-ai/foundry/internal/netguard"
	"github.com/orchis-ai/foundry/internal/scan"
)

// modelSettings loads a user's configured LLM provider creds (from the
// scanner_settings row). usable=true when a model is actually reachable
// (an API key for Anthropic, or a base URL for an OpenAI-compatible endpoint).
// Used by both the scanner (repo owner's model) and in-app AI (requester's model).
func (s *Server) modelSettings(ctx context.Context, userID int64) (scan.Settings, bool) {
	var st scan.Settings
	var enabled int
	err := s.db.QueryRowContext(ctx,
		`SELECT enabled, provider, base_url, model, api_key, context_budget, max_output_tokens, prune_globs
		 FROM scanner_settings WHERE user_id = ?`, userID).
		Scan(&enabled, &st.Provider, &st.BaseURL, &st.Model, &st.APIKey,
			&st.ContextBudget, &st.MaxOutputTokens, &st.PruneGlobs)
	if err != nil {
		return scan.Settings{}, false
	}
	st.Enabled = enabled == 1
	usable := (st.Provider == "anthropic" && st.APIKey != "") ||
		(st.Provider == "openai_compatible" && st.BaseURL != "")
	return st, usable
}

// GET /v1/me/scanner — returns the user's scanner config (api_key masked).
func (s *Server) handleGetScanner(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var enabled int
	var provider, baseURL, model, apiKey, pruneGlobs string
	var contextBudget, maxOutputTokens int
	err := s.db.QueryRowContext(r.Context(),
		`SELECT enabled, provider, base_url, model, api_key, context_budget, max_output_tokens, prune_globs
		 FROM scanner_settings WHERE user_id = ?`, u.ID).
		Scan(&enabled, &provider, &baseURL, &model, &apiKey, &contextBudget, &maxOutputTokens, &pruneGlobs)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false, "provider": "anthropic", "baseUrl": "", "model": "", "hasKey": false,
			"contextBudget": 8000, "maxOutputTokens": 2048, "pruneGlobs": "",
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load scanner settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":         enabled == 1,
		"provider":        provider,
		"baseUrl":         baseURL,
		"model":           model,
		"hasKey":          apiKey != "", // never return the key itself
		"contextBudget":   contextBudget,
		"maxOutputTokens": maxOutputTokens,
		"pruneGlobs":      pruneGlobs,
	})
}

// PUT /v1/me/scanner — upserts the user's scanner config.
// If apiKey is omitted/empty and a key already exists, the existing key is kept.
func (s *Server) handlePutScanner(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var in struct {
		Enabled         bool   `json:"enabled"`
		Provider        string `json:"provider"`
		BaseURL         string `json:"baseUrl"`
		Model           string `json:"model"`
		APIKey          string `json:"apiKey"`
		ContextBudget   int    `json:"contextBudget"`
		MaxOutputTokens int    `json:"maxOutputTokens"`
		PruneGlobs      string `json:"pruneGlobs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Provider != "anthropic" && in.Provider != "openai_compatible" {
		writeError(w, http.StatusBadRequest, "provider must be 'anthropic' or 'openai_compatible'")
		return
	}
	// SSRF guard: an OpenAI-compatible base URL is owner-supplied and gets
	// called server-side, so it must not point at internal/metadata addresses
	// (unless the operator opted in via ORCHIS_ALLOW_INTERNAL_TARGETS).
	if in.Provider == "openai_compatible" && in.BaseURL != "" {
		if err := netguard.ValidateOutboundURL(in.BaseURL); err != nil {
			writeError(w, http.StatusBadRequest, "base URL rejected: "+err.Error())
			return
		}
	}
	// Clamp the budgets to sane ranges (defaults when unset/out of range).
	if in.ContextBudget < 1000 || in.ContextBudget > 200000 {
		in.ContextBudget = 8000
	}
	if in.MaxOutputTokens < 256 || in.MaxOutputTokens > 32000 {
		in.MaxOutputTokens = 2048
	}

	// Preserve existing key when the client doesn't send a new one.
	key := in.APIKey
	if key == "" {
		var existing string
		s.db.QueryRowContext(r.Context(), `SELECT api_key FROM scanner_settings WHERE user_id = ?`, u.ID).Scan(&existing)
		key = existing
	}

	en := 0
	if in.Enabled {
		en = 1
	}
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO scanner_settings (user_id, enabled, provider, base_url, model, api_key, context_budget, max_output_tokens, prune_globs, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?, datetime('now'))
		 ON CONFLICT(user_id) DO UPDATE SET enabled=excluded.enabled, provider=excluded.provider,
		   base_url=excluded.base_url, model=excluded.model, api_key=excluded.api_key,
		   context_budget=excluded.context_budget, max_output_tokens=excluded.max_output_tokens,
		   prune_globs=excluded.prune_globs, updated_at=datetime('now')`,
		u.ID, en, in.Provider, in.BaseURL, in.Model, key, in.ContextBudget, in.MaxOutputTokens, in.PruneGlobs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save scanner settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": in.Enabled, "provider": in.Provider, "baseUrl": in.BaseURL, "model": in.Model, "hasKey": key != "",
		"contextBudget": in.ContextBudget, "maxOutputTokens": in.MaxOutputTokens, "pruneGlobs": in.PruneGlobs,
	})
}

// POST .../pulls/:num/scan — manually enqueue a scan for the PR head.
func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var headSHA string
	s.db.QueryRowContext(r.Context(), `SELECT head_sha FROM pulls WHERE id = ?`, id).Scan(&headSHA)
	s.scan.Enqueue(id, row.ID, headSHA)
	writeJSON(w, http.StatusOK, map[string]any{"queued": true})
}
