package render

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// MergeMCP merges pack-declared servers into an existing .mcp.json, removing
// keys in prevOwned that are no longer declared. Keys not owned by escapement
// are never touched. Returns the merged document and the sorted owned keys.
func MergeMCP(existing []byte, servers map[string]map[string]any, prevOwned []string) ([]byte, []string, error) {
	doc := map[string]any{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			return nil, nil, fmt.Errorf("parsing existing .mcp.json: %w", err)
		}
	}
	cur, _ := doc["mcpServers"].(map[string]any)
	if cur == nil {
		cur = map[string]any{}
	}
	for _, k := range prevOwned {
		if _, still := servers[k]; !still {
			delete(cur, k)
		}
	}
	owned := make([]string, 0, len(servers))
	for k, v := range servers {
		cur[k] = v
		owned = append(owned, k)
	}
	sort.Strings(owned)
	doc["mcpServers"] = cur
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(out, '\n'), owned, nil
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
	canon, err := json.Marshal(subset) // map keys marshal sorted
	if err != nil {
		return "", err
	}
	return esc.HashBytes(canon), nil
}

// DesiredMCPHash is OwnedMCPHash for the pack-declared servers themselves.
func DesiredMCPHash(servers map[string]map[string]any) (string, error) {
	subset := map[string]any{}
	for k, v := range servers {
		subset[k] = v
	}
	canon, err := json.Marshal(subset)
	if err != nil {
		return "", err
	}
	return esc.HashBytes(canon), nil
}
