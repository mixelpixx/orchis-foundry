package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/orchis-ai/foundry/internal/auth"
)

func (s *Server) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	keys, err := s.sessions.ListSSHKeys(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list keys")
		return
	}
	if keys == nil {
		keys = []auth.SSHKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) handleAddSSHKey(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var in struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	key, err := s.sessions.AddSSHKey(r.Context(), u.ID, in.Name, in.Key)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (s *Server) handleDeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.sessions.DeleteSSHKey(r.Context(), u.ID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
