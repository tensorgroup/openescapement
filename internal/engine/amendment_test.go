package engine

import "testing"

func TestNewAmendment(t *testing.T) {
	if got := newAmendment("", nil); got != nil {
		t.Errorf("empty content must yield nil, got %+v", got)
	}
	if got := newAmendment("   \n\n", nil); got != nil {
		t.Errorf("whitespace-only content must yield nil, got %+v", got)
	}
	got := newAmendment("one\ntwo\n", nil)
	if got == nil || got.Lines != 2 || got.Bytes != 8 {
		t.Fatalf("got %+v, want 2 lines / 8 bytes", got)
	}
	if got.Hash == "" {
		t.Error("hash must be set")
	}
	if got.Content != "one\ntwo\n" {
		t.Errorf("content = %q", got.Content)
	}
	items := newAmendment("", []string{"theirs"})
	if items == nil || len(items.Items) != 1 {
		t.Fatalf("items-only amendment must be non-nil, got %+v", items)
	}
}
