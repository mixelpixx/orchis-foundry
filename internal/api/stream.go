package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/orchis-ai/foundry/internal/auth"
)

// GET /v1/stream?topics=pull:org/name#3,repo:org/name,inbox:<uid>
//
// Server-Sent Events: keeps the connection open and streams events for the
// requested topics that the caller is allowed to see. Heartbeats every 25s as
// a `: ping` comment to defeat proxy idle timeouts. Auth is via the session
// cookie (EventSource can't set Authorization headers).
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r) // requireUser guarantees non-nil
	if s.rt == nil {
		writeError(w, http.StatusServiceUnavailable, "realtime not available")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	var allowed []string
	for _, t := range strings.Split(r.URL.Query().Get("topics"), ",") {
		t = strings.TrimSpace(t)
		if t != "" && s.canSubscribe(u, t) {
			allowed = append(allowed, t)
		}
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // tell nginx not to buffer the stream
	w.WriteHeader(http.StatusOK)

	sub := s.rt.Subscribe(allowed)
	defer s.rt.Unsubscribe(sub)

	fmt.Fprintf(w, ": connected (%d topics)\n\n", len(allowed))
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-sub.C:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// canSubscribe enforces ACL on a topic for the given user.
//   - inbox:<uid>      only your own inbox
//   - repo:<org>/<name> and pull:<org>/<name>#<num>  any repo you can read
func (s *Server) canSubscribe(u *auth.User, topic string) bool {
	colon := strings.IndexByte(topic, ':')
	if colon < 0 {
		return false
	}
	kind, body := topic[:colon], topic[colon+1:]
	switch kind {
	case "inbox":
		return body == strconv.FormatInt(u.ID, 10)
	case "repo", "pull":
		if i := strings.IndexByte(body, '#'); i >= 0 {
			body = body[:i]
		}
		parts := strings.SplitN(body, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return false
		}
		row, err := s.repoForOwnerName(parts[0], parts[1])
		if err != nil {
			return false
		}
		if row.Visibility == "public" {
			return true
		}
		return row.OwnerUserID.Valid && row.OwnerUserID.Int64 == u.ID
	}
	return false
}

// publish is a nil-safe shortcut for handlers.
func (s *Server) publish(topic, kind string, payload any) {
	if s.rt != nil {
		s.rt.Publish(topic, kind, payload)
	}
}

// topic builders keep the naming consistent across publishers.
func repoTopic(owner, name string) string { return "repo:" + owner + "/" + name }
func pullTopic(owner, name string, num int) string {
	return "pull:" + owner + "/" + name + "#" + strconv.Itoa(num)
}
