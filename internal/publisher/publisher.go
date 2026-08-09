package publisher

import "github.com/tensorgroup/openescapement/internal/engine"

// Envelope is the wire document a publisher transport sends: repo identity
// alongside the (possibly redacted, see Redact) Report. Defined here, ahead
// of the client that sends it, because Redact's test needs to marshal a
// Report exactly as production code will — inside the envelope, not bare —
// so a future field named "content" or "diff" added to Envelope itself
// falls under the same walk-test coverage as one added to Report. Task 5
// fills in the client that constructs and transmits an Envelope.
type Envelope struct {
	Schema     int            `json:"schema"`
	Remote     string         `json:"remote"`
	ConfigPath string         `json:"config_path"`
	Report     *engine.Report `json:"report"`
}
