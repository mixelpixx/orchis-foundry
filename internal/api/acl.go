package api

import (
	"context"
	"net/http"
)

// Role values for a user's relationship to a repo. Empty = no membership
// (may still have public read). "owner" is the repo's owner_user_id.
const (
	roleOwner = "owner"
	roleAdmin = "admin"
	roleWrite = "write"
	roleRead  = "read"
)

// collaboratorRole returns the stored collaborator role for (repo, user), or
// ("", false) if the user is not a collaborator.
func (s *Server) collaboratorRole(ctx context.Context, repoID, userID int64) (string, bool) {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM repo_collaborators WHERE repo_id = ? AND user_id = ?`, repoID, userID).Scan(&role)
	if err != nil {
		return "", false
	}
	return role, true
}

// roleFor computes the caller's effective role on a repo: owner, then a
// collaborator role, else "".
func (s *Server) roleFor(ctx context.Context, row *repoRow, userID int64) string {
	if row.OwnerUserID.Valid && row.OwnerUserID.Int64 == userID {
		return roleOwner
	}
	if r, ok := s.collaboratorRole(ctx, row.ID, userID); ok {
		return r
	}
	return ""
}

// canWrite reports whether row.Role grants push/merge/branch/release rights.
func canWrite(row *repoRow) bool {
	return row.Role == roleOwner || row.Role == roleAdmin || row.Role == roleWrite
}

// canAdmin reports whether row.Role grants settings/delete/webhooks/collaborator
// management.
func canAdmin(row *repoRow) bool {
	return row.Role == roleOwner || row.Role == roleAdmin
}

// requireWrite / requireAdmin are handler guards used after loadRepo (which has
// already populated row.Role for the current user).
func (s *Server) requireWrite(w http.ResponseWriter, row *repoRow) bool {
	if !canWrite(row) {
		writeError(w, http.StatusForbidden, "you need write access to this repository")
		return false
	}
	return true
}

func (s *Server) requireAdmin(w http.ResponseWriter, row *repoRow) bool {
	if !canAdmin(row) {
		writeError(w, http.StatusForbidden, "you need admin access to this repository")
		return false
	}
	return true
}
