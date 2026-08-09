package render

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestUnownedMCPServers(t *testing.T) {
	raw := []byte(`{"mcpServers":{"ours":{"command":"a"},"theirs":{"command":"b"},"also":{"command":"c"}}}`)
	got, err := UnownedMCPServers(raw, []string{"ours"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"also", "theirs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if got, err := UnownedMCPServers(nil, nil); err != nil || len(got) != 0 {
		t.Errorf("empty file: got %v err %v", got, err)
	}
}

func TestMergeMCPShapeMatrix(t *testing.T) {
	servers := map[string]map[string]any{"esc-tools": {"command": "esc-mcp"}}
	cases := []struct {
		name     string
		existing string
		wantErr  bool
	}{
		{"nil", "", false},
		{"whitespace-only", "\n  \n", false}, // FAILS pre-fix: json parse error on whitespace
		{"empty-object", "{}\n", false},
		{"user-server-preserved", `{"mcpServers":{"team-db":{"command":"db"}}}`, false},
		{"crlf-json", "{\r\n  \"mcpServers\": {}\r\n}\r\n", false},
		{"no-trailing-newline", `{"mcpServers":{}}`, false},
		{"invalid-json", "not json", true},
		{"mcpservers-not-object", `{"mcpServers": 3}`, true},
		{"name-squat", `{"mcpServers":{"esc-tools":{"command":"theirs"}}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, keys, err := MergeMCP([]byte(tc.existing), servers, nil)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("unsupported shape must fail closed, got:\n%s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("MergeMCP: %v", err)
			}
			var doc map[string]any
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatalf("output is not valid JSON: %v\n%s", err, out)
			}
			ms, ok := doc["mcpServers"].(map[string]any)
			if !ok {
				t.Fatalf("output mcpServers is not an object:\n%s", out)
			}
			if _, ok := ms["esc-tools"]; !ok {
				t.Errorf("owned server missing")
			}
			if tc.name == "user-server-preserved" {
				if _, ok := ms["team-db"]; !ok {
					t.Errorf("user's own server entry was dropped")
				}
			}
			// The subsequent sync: merging again over our own output must
			// be a byte-for-byte fixed point.
			again, _, err := MergeMCP(out, servers, keys)
			if err != nil {
				t.Fatalf("second merge (sync) errored: %v", err)
			}
			if !bytes.Equal(again, out) {
				t.Errorf("MergeMCP not idempotent:\nfirst  %s\nsecond %s", out, again)
			}
		})
	}
}
