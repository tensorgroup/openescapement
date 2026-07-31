// Package charts renders deterministic, server-side SVG charts with no
// client-side JavaScript. Output is safe to embed directly into HTML
// templates via template.HTML.
package charts

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"
)

// Point is a single (time, value) sample for a line chart.
type Point struct {
	X time.Time
	Y float64
}

// Series is one named series of values for a stacked bar chart. Values is
// positional: Values[i] corresponds to labels[i] passed to StackedBars.
type Series struct {
	Label  string
	Values []float64 // one per label slot; len(Values) == len(labels)
}

var palette = []string{"#2563eb", "#1a7a4f", "#9a560f", "#7c3aed", "#b02a2a", "#3b82a0"}

const (
	mLeft, mRight, mTop, mBottom = 40.0, 20.0, 10.0, 24.0
	textAttrs                    = `font-family="system-ui,sans-serif" font-size="11" fill="#5b6672"`
	gridStroke                   = "#e9ecf1"
)

// gridLevels returns the y-values to draw gridlines/labels at: quarters from
// 0 to maxY (five lines), for a calmer, easier-to-read grid than 0/mid/max.
func gridLevels(maxY float64) []float64 {
	return []float64{0, maxY / 4, maxY / 2, 3 * maxY / 4, maxY}
}

func f(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func abbrev(v float64) string {
	switch {
	case v >= 1e6:
		return strconv.FormatFloat(v/1e6, 'f', 1, 64) + "M"
	case v >= 1e3:
		return strconv.FormatFloat(v/1e3, 'f', 1, 64) + "k"
	default:
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
}

// Abbrev abbreviates large numbers for display (e.g. 12300 -> "12.3k",
// 4500000 -> "4.5M"). Exported for use as a template function.
func Abbrev(v float64) string { return abbrev(v) }

func niceCeil(v float64) float64 {
	if v <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(v)) - 1
	step := math.Pow(10, exp)
	return math.Ceil(v/step) * step
}

func esc(s string) string { return template.HTMLEscapeString(s) }

func empty(w, h int) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img"><text x="%s" y="%s" text-anchor="middle" %s>No data yet</text></svg>`,
		w, h, w, h, f(float64(w)/2), f(float64(h)/2), textAttrs))
}

// Line renders pts as a single polyline SVG chart of width w and height h.
// Empty input returns a styled "No data yet" placeholder of the same size.
func Line(pts []Point, w, h int) template.HTML {
	if len(pts) == 0 {
		return empty(w, h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img">`, w, h, w, h)
	maxY := 0.0
	for _, p := range pts {
		maxY = math.Max(maxY, p.Y)
	}
	maxY = niceCeil(maxY)
	pw := float64(w) - mLeft - mRight
	ph := float64(h) - mTop - mBottom
	// gridlines + y labels at quarter intervals
	for _, v := range gridLevels(maxY) {
		y := mTop + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y), gridStroke)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, esc(abbrev(v)))
	}
	var poly []string
	for i, p := range pts {
		x := mLeft
		if len(pts) > 1 {
			x = mLeft + pw*float64(i)/float64(len(pts)-1)
		}
		y := mTop + ph - ph*p.Y/maxY
		poly = append(poly, f(x)+","+f(y))
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`,
		strings.Join(poly, " "), palette[0])
	// first/last x labels
	fmt.Fprintf(&b, `<text x="%s" y="%s" %s>%s</text>`,
		f(mLeft), f(float64(h)-6), textAttrs, esc(pts[0].X.Format("Jan 2")))
	fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
		f(mLeft+pw), f(float64(h)-6), textAttrs, esc(pts[len(pts)-1].X.Format("Jan 2")))
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// StackedBars renders series as a stacked bar chart, one bar per label
// slot, series stacked bottom-up in series order. A legend listing series
// labels appears along the top. Each bar segment carries a <title> child
// for native hover tooltips. Empty input returns a styled "No data yet"
// placeholder of the same size.
func StackedBars(labels []string, series []Series, w, h int) template.HTML {
	if len(labels) == 0 || len(series) == 0 {
		return empty(w, h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img">`, w, h, w, h)

	// Legend row at top: colored square + label, advancing left to right.
	legendY := 10.0
	lx := mLeft
	for i, s := range series {
		color := palette[i%len(palette)]
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="10" height="10" fill="%s"/>`,
			f(lx), f(legendY-9), color)
		fmt.Fprintf(&b, `<text x="%s" y="%s" %s>%s</text>`,
			f(lx+14), f(legendY), textAttrs, esc(s.Label))
		lx += 14 + 7*float64(len(s.Label)) + 12
	}

	top := mTop + 20
	pw := float64(w) - mLeft - mRight
	ph := float64(h) - top - mBottom

	// max column sum across label slots
	maxY := 0.0
	for i := range labels {
		sum := 0.0
		for _, s := range series {
			if i < len(s.Values) {
				sum += s.Values[i]
			}
		}
		maxY = math.Max(maxY, sum)
	}
	maxY = niceCeil(maxY)

	// gridlines + y labels at quarter intervals
	for _, v := range gridLevels(maxY) {
		y := top + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y), gridStroke)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, esc(abbrev(v)))
	}

	slotW := pw / float64(len(labels))
	barW := slotW * 0.7

	for i, label := range labels {
		x := mLeft + slotW*float64(i) + slotW*0.15
		fmt.Fprintf(&b, `<g>`)
		yCursor := top + ph
		for si, s := range series {
			var v float64
			if i < len(s.Values) {
				v = s.Values[i]
			}
			barH := ph * v / maxY
			y := yCursor - barH
			color := palette[si%len(palette)]
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"><title>%s</title></rect>`,
				f(x), f(y), f(barW), f(barH), color,
				esc(fmt.Sprintf("%s %s: %s", label, s.Label, abbrev(v))))
			yCursor = y
		}
		b.WriteString(`</g>`)
	}

	// x labels: every Nth label where N = ceil(len/8)
	n := int(math.Ceil(float64(len(labels)) / 8))
	if n < 1 {
		n = 1
	}
	for i, label := range labels {
		if i%n != 0 {
			continue
		}
		x := mLeft + slotW*float64(i) + slotW/2
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" %s>%s</text>`,
			f(x), f(float64(h)-6), textAttrs, esc(label))
	}

	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
