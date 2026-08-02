package guidance

import "testing"

func TestParseEmbeddedRegistry(t *testing.T) {
	data, err := embeddedModels.ReadFile("models/models.yaml")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := ParseRegistry(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var keys []string
	for _, v := range reg.Vendors {
		keys = append(keys, v.Key)
	}
	want := []string{"anthropic", "openai", "google", "kimi", "deepseek", "grok"}
	if len(keys) != len(want) {
		t.Fatalf("vendors = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, keys[i], want[i])
		}
	}
}

func TestParseRegistryReordersAndRejects(t *testing.T) {
	valid := "vendors:\n  - {key: google, name: Google, models: []}\n  - {key: anthropic, name: Anthropic, models: []}\n"
	reg, err := ParseRegistry([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if reg.Vendors[0].Key != "anthropic" || reg.Vendors[1].Key != "google" {
		t.Fatalf("not reordered: %v", reg.Vendors)
	}
	cases := map[string]string{
		"unknown vendor":                    "vendors:\n  - {key: acme, name: Acme, models: []}\n",
		"unknown field":                     "vendors:\n  - {key: anthropic, name: A, bogus: 1, models: []}\n",
		"bad tier":                          "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: huge, roles: [coding], status: current, docs: [], verified: 2026-08-02}]\n",
		"bad role":                          "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [wizard], status: current, docs: [], verified: 2026-08-02}]\n",
		"bad status":                        "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: retired, docs: [], verified: 2026-08-02}]\n",
		"bad date":                          "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: current, docs: [], verified: yesterday}]\n",
		"duplicate model id one vendor":     "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: current, docs: [], verified: 2026-08-02}, {id: x, name: X2, tier: mid, roles: [coding], status: current, docs: [], verified: 2026-08-02}]\n",
		"duplicate model id across vendors": "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: current, docs: [], verified: 2026-08-02}]\n  - key: openai\n    name: O\n    models: [{id: x, name: X2, tier: mid, roles: [coding], status: current, docs: [], verified: 2026-08-02}]\n",
		"empty roles list":                  "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [], status: current, docs: [], verified: 2026-08-02}]\n",
	}
	for name, src := range cases {
		if _, err := ParseRegistry([]byte(src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
