// Package guidance owns the curated, sourced model-guidance tree: a strict
// models.yaml registry plus per-vendor markdown notes and example rule-pack
// fragments. The embedded copy is the seed; esc serve seeds it to
// <data-dir>/guidance/ and the portal reads that on every request.
package guidance

import (
	"bytes"
	"embed"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed models
var embeddedModels embed.FS

// VendorOrder is the fixed display order (ICP-relevant vendors first). The
// registry always renders vendors in this order regardless of file order.
var VendorOrder = []string{"anthropic", "openai", "google", "kimi", "deepseek", "grok"}

var vendorName = map[string]string{
	"anthropic": "Anthropic", "openai": "OpenAI", "google": "Google",
	"kimi": "Kimi", "deepseek": "Deepseek", "grok": "Grok",
}

var (
	validTier   = map[string]bool{"frontier": true, "mid": true, "fast": true}
	validRole   = map[string]bool{"planning": true, "plan-check": true, "review": true, "coding": true, "bulk": true}
	validStatus = map[string]bool{"current": true, "legacy": true}
)

// Model is one model's registry entry. ID matches usage-telemetry model ids.
type Model struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Tier     string   `yaml:"tier"`
	Roles    []string `yaml:"roles"`
	Status   string   `yaml:"status"`
	Docs     []string `yaml:"docs"`
	Verified string   `yaml:"verified"`
}

// Vendor groups a vendor's models. Name is normalized from the canonical map.
type Vendor struct {
	Key    string  `yaml:"key"`
	Name   string  `yaml:"name"`
	Models []Model `yaml:"models"`
}

// Registry is the parsed models.yaml.
type Registry struct {
	Vendors []Vendor `yaml:"vendors"`
}

// ParseRegistry strictly parses and validates models.yaml, returning vendors
// in VendorOrder. Unknown fields, unknown/duplicate vendor keys, invalid
// enums, and unparseable verified dates are errors.
func ParseRegistry(data []byte) (*Registry, error) {
	var reg Registry
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&reg); err != nil {
		return nil, fmt.Errorf("guidance: parsing models.yaml: %w", err)
	}
	seen := map[string]bool{}
	byKey := map[string]Vendor{}
	seenModelID := map[string]bool{}
	for _, v := range reg.Vendors {
		if _, ok := vendorName[v.Key]; !ok {
			return nil, fmt.Errorf("guidance: unknown vendor %q", v.Key)
		}
		if seen[v.Key] {
			return nil, fmt.Errorf("guidance: duplicate vendor %q", v.Key)
		}
		seen[v.Key] = true
		for _, m := range v.Models {
			if m.ID == "" {
				return nil, fmt.Errorf("guidance: vendor %s: model missing id", v.Key)
			}
			if seenModelID[m.ID] {
				return nil, fmt.Errorf("guidance: duplicate model id %q", m.ID)
			}
			seenModelID[m.ID] = true
			if !validTier[m.Tier] {
				return nil, fmt.Errorf("guidance: model %s: invalid tier %q", m.ID, m.Tier)
			}
			if !validStatus[m.Status] {
				return nil, fmt.Errorf("guidance: model %s: invalid status %q", m.ID, m.Status)
			}
			if len(m.Roles) == 0 {
				return nil, fmt.Errorf("guidance: model %s: roles must not be empty", m.ID)
			}
			for _, role := range m.Roles {
				if !validRole[role] {
					return nil, fmt.Errorf("guidance: model %s: invalid role %q", m.ID, role)
				}
			}
			if _, err := time.Parse("2006-01-02", m.Verified); err != nil {
				return nil, fmt.Errorf("guidance: model %s: invalid verified date %q", m.ID, m.Verified)
			}
		}
		v.Name = vendorName[v.Key]
		byKey[v.Key] = v
	}
	ordered := make([]Vendor, 0, len(byKey))
	for _, key := range VendorOrder {
		if v, ok := byKey[key]; ok {
			ordered = append(ordered, v)
		}
	}
	reg.Vendors = ordered
	return &reg, nil
}
