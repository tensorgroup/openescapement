package web

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/portal/store"
	"github.com/tensorgroup/openescapement/internal/publisher"
)

// maxEventBody caps a single ingested event's JSON body at 64KB.
const maxEventBody = 64 << 10

// handleEvents accepts either shape of ingest body, distinguished by the
// presence of a top-level "schema" key:
//
//   - A publisher.Envelope (the versioned wire document esc's publisher
//     client sends): see handleEnvelope.
//   - A raw store.Event (pre-envelope, still emitted by older publisher
//     builds and portal tests): the original behavior, unchanged. Unknown
//     fields are tolerated (plain json.Unmarshal). kind must pass
//     store.ValidKind; a zero ts is stamped with the server's clock.
//
// Auth follows the same policy as the page routes (withAuth wraps this
// handler too).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEventBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed json")
		return
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed json")
		return
	}
	if _, isEnvelope := probe["schema"]; isEnvelope {
		s.handleEnvelope(w, body)
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
		log.Printf("portal: internal error: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// handleEnvelope decodes body as a publisher.Envelope and builds the
// store.Event it represents.
//
//   - Schema must equal publisher.EnvelopeSchema; any other value is a 400
//     naming the expected version, since ingest cannot safely guess how an
//     unknown schema maps to store.Event.
//   - Report.Command becomes Event.Kind (already a valid store.ValidKind
//     value: "sync"/"status").
//   - Report.Packs maps to Event.Packs: EventPack.Name <- ReportPack.Source,
//     EventPack.Version <- ReportPack.Pinned (the resolved content hash, the
//     one identifier every pin always carries, since Ref is empty for local
//     packs), EventPack.Signed <- ReportPack.Signed.
//   - Report.Findings maps to Event.Artifacts for artifact-kind findings
//     only (block/file/dir/json-keys); KindPack/KindUpdateCheck/
//     KindConstraint findings describe pack pins and subsystems, not
//     on-disk artifacts, and are skipped.
//   - Event.Drift is derived from the mapped artifacts (store.DeriveDrift),
//     never taken from the wire.
//   - Envelope.Remote is normalized (publisher.NormalizeRemote) and always
//     recorded on Event.Remote. A registry match additionally stamps
//     RepoID/TeamID; an unmatched remote is retained rather than dropped,
//     since it is the fleet's shadow-IT view.
//   - TS is stamped server-side, same as the raw-Event path.
func (s *Server) handleEnvelope(w http.ResponseWriter, body []byte) {
	var env publisher.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed json")
		return
	}
	if env.Schema != publisher.EnvelopeSchema {
		writeJSONError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported envelope schema %d, want %d", env.Schema, publisher.EnvelopeSchema))
		return
	}
	if env.Report == nil {
		writeJSONError(w, http.StatusBadRequest, "missing report")
		return
	}
	rep := env.Report
	if !store.ValidKind(rep.Command) {
		writeJSONError(w, http.StatusBadRequest, "unknown kind")
		return
	}

	e := store.Event{Kind: rep.Command}
	for _, p := range rep.Packs {
		e.Packs = append(e.Packs, store.EventPack{
			Name:    p.Source,
			Version: p.Pinned,
			Signed:  p.Signed,
		})
	}
	for _, f := range rep.Findings {
		if !isArtifactKind(f.Kind) {
			continue
		}
		e.Artifacts = append(e.Artifacts, store.EventArtifact{
			Path:       f.Subject,
			Kind:       f.Kind,
			Managed:    string(f.State),
			Local:      string(f.Local),
			Amendment:  f.Amendment,
			Alteration: f.Alteration,
		})
	}
	coll := rep.Collection
	e.Collection = &coll
	e.Drift = store.DeriveDrift(e.Artifacts)

	remote := publisher.NormalizeRemote(env.Remote)
	e.Remote = remote
	if repo, ok := s.Store.Registry().RepoByRemote(remote); ok {
		e.RepoID = repo.ID
		e.TeamID = repo.TeamID
	}

	e.TS = s.Now()

	if err := s.Store.AppendEvent(e); err != nil {
		log.Printf("portal: internal error: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// isArtifactKind reports whether k is one of Finding's on-disk artifact
// kinds (block/file/dir/json-keys), as opposed to KindPack/KindUpdateCheck/
// KindConstraint, which describe pack pins and subsystems rather than
// artifacts and have no EventArtifact representation.
func isArtifactKind(k string) bool {
	switch k {
	case engine.KindBlock, engine.KindFile, engine.KindDir, engine.KindJSONKeys:
		return true
	default:
		return false
	}
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
