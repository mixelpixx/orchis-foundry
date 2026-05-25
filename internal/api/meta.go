package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openapiJSON []byte

// GET /llms.txt — machine-readable index for LLM agents (llms.txt convention).
func (s *Server) handleLLMsTxt(w http.ResponseWriter, r *http.Request) {
	base := s.cfg.ExternalURL
	body := "# Orchis Foundry\n\n" +
		"A self-hosted code platform (git hosting, pull requests, per-user BYO-model supply-chain scanning).\n" +
		"This instance: " + base + "\n\n" +
		"## API\n\n" +
		"- Base URL: " + base + "/v1\n" +
		"- Auth: send `Authorization: Bearer orc_pat_…` (create a token in the UI under Developer settings → Tokens, or `POST /v1/me/tokens`).\n" +
		"- Machine-readable spec: " + base + "/v1/openapi.json (OpenAPI 3.1)\n\n" +
		"## LLM-friendly endpoints\n\n" +
		"- `GET /v1/repos/{owner}/{repo}/pack?ref=&maxBytes=` — the whole repo as one markdown document, ready to paste into a model.\n" +
		"- `GET /v1/repos/{owner}/{repo}/pulls/{num}/pack` — a pull request (description + commits + diff + comments) as one document.\n" +
		"- `GET /v1/repos/{owner}/{repo}/tree?ref=&path=`, `/blob?path=`, `/raw?path=`, `/readme` — structured read access.\n\n" +
		"## Writing code\n\n" +
		"- `POST /v1/repos/{owner}/{repo}/commits {branch, baseSha?, message, changes:[{path, content|delete}]}` commits edits (needs write access).\n" +
		"- `GET /v1/repos/{owner}/{repo}/blame?ref=&path=` returns per-line authorship; `GET .../outline?ref=&path=` returns structural symbols.\n" +
		"- AI-proposed changes go through a human gate: `POST .../proposals {title, patch}` then a reviewer accepts via `POST .../proposals/{id}/accept`.\n" +
		"- `POST .../commits/draft-message` generates a Conventional-Commits message from a diff.\n\n" +
		"## Working with a PR\n\n" +
		"- List: `GET /v1/repos/{owner}/{repo}/pulls?state=open` ; detail: `.../pulls/{num}` ; diff: `.../pulls/{num}/files`.\n" +
		"- Open: `POST /v1/repos/{owner}/{repo}/pulls {title,head,base,body}`.\n" +
		"- Comment / review / merge: `POST .../pulls/{num}/{comments,reviews,merge}`.\n" +
		"- In-app model help (uses your configured model): `POST .../pulls/{num}/{summarize,explain,draft-description}`.\n\n" +
		"## Git\n\n" +
		"- Clone/push over HTTPS: `git clone " + base + "/{owner}/{repo}.git` (Basic auth: handle + PAT).\n" +
		"- Or SSH on port 2222 with a registered key.\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body))
}

// GET /v1/openapi.json — embedded OpenAPI 3.1 spec.
func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(openapiJSON)
}
