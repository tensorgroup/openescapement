package charts

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s", name, got)
	}
}

func TestLineGolden(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	var pts []Point
	for i := 0; i < 10; i++ {
		pts = append(pts, Point{X: base.AddDate(0, 0, i), Y: float64(3 + i*2)})
	}
	golden(t, "line.golden.svg", string(Line(pts, 480, 160)))
}

func TestStackedBarsGolden(t *testing.T) {
	labels := []string{"Jul 1", "Jul 2", "Jul 3"}
	series := []Series{
		{Label: "claude-sonnet-5", Values: []float64{1200, 3400, 2100}},
		{Label: "claude-haiku-4-5", Values: []float64{800, 600, 1500}},
	}
	golden(t, "bars.golden.svg", string(StackedBars(labels, series, 480, 200)))
}

func TestEmptyStates(t *testing.T) {
	if got := string(Line(nil, 480, 160)); !contains(got, "No data yet") {
		t.Fatalf("line empty state: %s", got)
	}
	if got := string(StackedBars(nil, nil, 480, 200)); !contains(got, "No data yet") {
		t.Fatalf("bars empty state: %s", got)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
