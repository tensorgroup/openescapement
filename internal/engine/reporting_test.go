package engine

import (
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/pack"
)

func pk(level string) *pack.Pack {
	p := &pack.Pack{}
	if level != "" {
		p.Manifest.Reporting = &pack.Reporting{Amendments: level}
	}
	return p
}

func TestResolveReporting(t *testing.T) {
	cases := []struct {
		name       string
		packs      []*pack.Pack
		override   string
		wantLevel  string
		wantSource string
		wantErr    bool
	}{
		{"no packs declare", []*pack.Pack{pk("")}, "", "off", "default", false},
		{"single pack metrics", []*pack.Pack{pk("metrics")}, "", "metrics", "pack", false},
		{"single pack content", []*pack.Pack{pk("content")}, "", "content", "pack", false},
		{"highest wins", []*pack.Pack{pk("metrics"), pk("content")}, "", "content", "pack", false},
		{"highest wins reversed", []*pack.Pack{pk("content"), pk("metrics")}, "", "content", "pack", false},
		{"repo clamps down", []*pack.Pack{pk("content")}, "metrics", "metrics", "repo-override", false},
		{"repo clamps off", []*pack.Pack{pk("content")}, "off", "off", "repo-override", false},
		{"repo cannot raise", []*pack.Pack{pk("metrics")}, "content", "", "", true},
		{"invalid pack level", []*pack.Pack{pk("everything")}, "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveReporting(tc.packs, &config.Config{ReportAmendments: tc.override})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Amendments != tc.wantLevel || got.Source != tc.wantSource {
				t.Errorf("got %+v, want %s/%s", got, tc.wantLevel, tc.wantSource)
			}
		})
	}
}
