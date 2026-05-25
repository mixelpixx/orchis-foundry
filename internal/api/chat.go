package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// The conversational chat sidebar (per-user, toggleable via the `aiChat`
// preference). Context is assembled from files the user pinned in the tree,
// run through the same pruning + budget controls as the rest of the platform.

const chatPerFileMax = 64 * 1024

// buildPinnedContext reads the pinned files at HEAD, skips pruned/binary/oversized
// ones, and concatenates them under a byte budget for the model prompt.
func (s *Server) buildPinnedContext(repoID int64, pinned []string, extraGlobs []string, budget int) string {
	if len(pinned) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range pinned {
		if shouldPrune(p, extraGlobs) {
			continue
		}
		blob, err := s.git.FileBlob(repoID, "", p)
		if err != nil || blob == nil {
			continue
		}
		if blob.Size > chatPerFileMax || isBinary(blob.Content) {
			continue
		}
		if b.Len()+len(blob.Content) > budget {
			break
		}
		b.WriteString("### " + p + "\n```" + blob.Lang + "\n" + blob.Content)
		if !strings.HasSuffix(blob.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}
	return b.String()
}

// POST /v1/repos/{org}/{name}/chat
// { chatId?, messages:[{role,content}], pinned:[paths] }  → { chatId, message }
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	st, usable := s.modelSettings(r.Context(), u.ID)
	if !usable {
		writeError(w, http.StatusConflict, "no model configured — set one up in Developer settings → Scanner")
		return
	}
	var in struct {
		ChatID  int64  `json:"chatId"`
		Pinned  []string `json:"pinned"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(in.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages required")
		return
	}
	last := in.Messages[len(in.Messages)-1]
	if last.Role != "user" || strings.TrimSpace(last.Content) == "" {
		writeError(w, http.StatusBadRequest, "last message must be a non-empty user turn")
		return
	}

	// Assemble context from pinned files (pruned + budgeted).
	ctxBlock := s.buildPinnedContext(row.ID, in.Pinned, parsePruneGlobs(st.PruneGlobs), st.InputBudgetBytes())
	system := "You are a coding assistant embedded in the Orchis Foundry repository " +
		row.OwnerHandle + "/" + row.Name + ". Answer concisely in markdown. " +
		"If you propose code changes, describe them clearly; the user applies changes through the review gate, not by you writing files directly."
	if ctxBlock != "" {
		system += "\n\nThe user pinned these files for context:\n\n" + ctxBlock
	}

	// Flatten the prior turns into the user prompt (single-dispatch provider).
	var convo strings.Builder
	for _, m := range in.Messages {
		role := "User"
		if m.Role == "assistant" {
			role = "Assistant"
		}
		convo.WriteString(role + ": " + m.Content + "\n\n")
	}
	convo.WriteString("Assistant:")

	out, code, msg := runChat(r.Context(), st, system, convo.String())
	if code != 0 {
		writeError(w, code, msg)
		return
	}

	// Persist: create or reuse the chat, append the user turn + assistant reply.
	chatID := in.ChatID
	if chatID == 0 {
		title := last.Content
		if len(title) > 60 {
			title = title[:60]
		}
		res, _ := s.db.ExecContext(r.Context(),
			`INSERT INTO ai_chats (user_id, repo_id, title) VALUES (?,?,?)`, u.ID, row.ID, title)
		chatID, _ = res.LastInsertId()
	} else {
		// Verify ownership before appending.
		var owner int64
		s.db.QueryRowContext(r.Context(), `SELECT user_id FROM ai_chats WHERE id = ? AND repo_id = ?`, chatID, row.ID).Scan(&owner)
		if owner != u.ID {
			writeError(w, http.StatusForbidden, "not your chat")
			return
		}
		s.db.ExecContext(r.Context(), `UPDATE ai_chats SET updated_at = datetime('now') WHERE id = ?`, chatID)
	}
	s.db.ExecContext(r.Context(), `INSERT INTO ai_chat_messages (chat_id, role, content) VALUES (?, 'user', ?)`, chatID, last.Content)
	s.db.ExecContext(r.Context(), `INSERT INTO ai_chat_messages (chat_id, role, content) VALUES (?, 'assistant', ?)`, chatID, out)

	writeJSON(w, http.StatusOK, map[string]any{"chatId": chatID, "message": out})
}

// GET /v1/repos/{org}/{name}/chats — the user's chats for this repo.
func (s *Server) handleListChats(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT id, title, updated_at FROM ai_chats WHERE user_id = ? AND repo_id = ? ORDER BY updated_at DESC LIMIT 50`, u.ID, row.ID)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var title, updated string
			if rows.Scan(&id, &title, &updated) == nil {
				out = append(out, map[string]any{"id": id, "title": title, "when": relativeTime(updated)})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/{org}/{name}/chats/{id} — messages for one chat.
func (s *Server) handleGetChat(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	chatID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var owner int64
	if s.db.QueryRowContext(r.Context(), `SELECT user_id FROM ai_chats WHERE id = ? AND repo_id = ?`, chatID, row.ID).Scan(&owner) != nil || owner != u.ID {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT role, content, created_at FROM ai_chat_messages WHERE chat_id = ? ORDER BY id`, chatID)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var role, content, created string
			if rows.Scan(&role, &content, &created) == nil {
				out = append(out, map[string]any{"role": role, "content": content})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": chatID, "messages": out})
}
