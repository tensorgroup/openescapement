package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// MergeMCP merges pack-declared servers into an existing .mcp.json, removing
// keys in prevOwned that are no longer declared. Keys not owned by escapement
// are never touched — a pack that collides with a user's existing server
// entry is an error, not a silent overwrite. Returns the merged document and
// the sorted owned keys.
func MergeMCP(existing []byte, servers map[string]map[string]any, prevOwned []string) ([]byte, []string, error) {
	doc := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			return nil, nil, fmt.Errorf("parsing existing .mcp.json: %w", err)
		}
	}
	cur := map[string]any{}
	if raw, present := doc["mcpServers"]; present {
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf(".mcp.json: mcpServers is not an object; refusing to overwrite it")
		}
		cur = obj
	}
	owned := map[string]bool{}
	for _, k := range prevOwned {
		owned[k] = true
		if _, still := servers[k]; !still {
			delete(cur, k)
		}
	}
	keys := make([]string, 0, len(servers))
	for k, v := range servers {
		if _, exists := cur[k]; exists && !owned[k] {
			return nil, nil, fmt.Errorf(".mcp.json: server %q already exists and is not managed by escapement; rename or remove it before syncing", k)
		}
		cur[k] = v
		keys = append(keys, k)
	}
	sort.Strings(keys)
	doc["mcpServers"] = cur
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(out, '\n'), keys, nil
}

// CanonicalServers serializes server entries deterministically (JSON with
// sorted keys) — the canonical form used for hashing and for constraint
// validation of MCP payloads.
func CanonicalServers(servers any) ([]byte, error) {
	return json.Marshal(servers)
}

// OwnedMCPHash returns the canonical hash of the owned server entries as
// they appear in a merged document. Used for drift detection on .mcp.json.
func OwnedMCPHash(merged []byte, owned []string) (string, error) {
	doc := map[string]any{}
	if len(merged) > 0 {
		if err := json.Unmarshal(merged, &doc); err != nil {
			return "", fmt.Errorf("parsing .mcp.json: %w", err)
		}
	}
	cur, _ := doc["mcpServers"].(map[string]any)
	subset := map[string]any{}
	for _, k := range owned {
		if v, ok := cur[k]; ok {
			subset[k] = v
		}
	}
	canon, err := CanonicalServers(subset)
	if err != nil {
		return "", err
	}
	return esc.HashBytes(canon), nil
}

// DesiredMCPHash is OwnedMCPHash for the pack-declared servers themselves.
// The values are JSON round-tripped first so YAML-decoded and JSON-decoded
// numbers hash identically.
func DesiredMCPHash(servers map[string]map[string]any) (string, error) {
	subset := map[string]any{}
	for k, v := range servers {
		subset[k] = v
	}
	canon, err := CanonicalServers(subset)
	if err != nil {
		return "", err
	}
	var roundTripped map[string]any
	if err := json.Unmarshal(canon, &roundTripped); err != nil {
		return "", err
	}
	canon, err = CanonicalServers(roundTripped)
	if err != nil {
		return "", err
	}
	return esc.HashBytes(canon), nil
}

// UnownedMCPServers returns the sorted names of server entries in file that
// are not in owned. A missing or empty file has none.
func UnownedMCPServers(file []byte, owned []string) ([]string, error) {
	if len(bytes.TrimSpace(file)) == 0 {
		return nil, nil
	}
	doc := map[string]any{}
	if err := json.Unmarshal(file, &doc); err != nil {
		return nil, fmt.Errorf("parsing .mcp.json: %w", err)
	}
	cur, _ := doc["mcpServers"].(map[string]any)
	ownedSet := make(map[string]bool, len(owned))
	for _, k := range owned {
		ownedSet[k] = true
	}
	var out []string
	for name := range cur {
		if !ownedSet[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}
