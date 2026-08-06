package render

import (
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
