package web

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

// maxEventBody caps a single ingested event's JSON body at 64KB.
const maxEventBody = 64 << 10

// handleEvents accepts a single store.Event as JSON. Unknown fields are
// tolerated (plain json.Unmarshal). kind must pass store.ValidKind; a zero
// ts is stamped with the server's clock. Auth follows the same policy as
// the page routes (withAuth wraps this handler too).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEventBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed json")
		return
	}

	var e store.Event
	if err := json.Unmarshal(body, &e); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed json")
		return
	}
	if !store.ValidKind(e.Kind) {
		writeJSONError(w, http.StatusBadRequest, "unknown kind")
		return
	}
	if e.TS.IsZero() {
		e.TS = s.Now()
	}

	if err := s.Store.AppendEvent(e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
