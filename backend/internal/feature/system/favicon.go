package system

import (
	_ "embed"
	"fmt"
	"math"
	"net/http"
	"regexp"
)

//go:embed logo.svg
var logoSVG string

// the single logo path (viewBox 0 0 1046 745), extracted once at startup
var logoPath = regexp.MustCompile(`(?s)\sd="([^"]*)"`).FindStringSubmatch(logoSVG)[1]

// logo transform to centre the 1046x745 mark in a 100x100 favicon with padding.
// for scale s the centring translate is ((100-1046*s)/2, (100-745*s)/2); this
// scale fills ~80% of the icon (~10px padding) so the mark reads bigger.
const logoTransform = "translate(10 21.5) scale(0.0765)"

// Favicon renders the app icon: a rounded square in the accent colour with the
// logo centred in the contrast colour. Public so the tab icon matches the theme.
func (h *Theme) Favicon(w http.ResponseWriter, r *http.Request) {
	accent := h.accent(r.Context())
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">`+
			`<rect width="100" height="100" rx="24" fill="%s"/>`+
			`<path transform="%s" fill="%s" d="%s"/></svg>`,
		accent, logoTransform, readableTextOn(accent), logoPath)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(svg))
}

// readableTextOn picks near-white or near-black for contrast on `accent`, using
// WCAG relative luminance (mirrors the frontend theme helper).
func readableTextOn(accent string) string {
	r, g, b, ok := parseHexColor(accent)
	if !ok {
		return "#ffffff"
	}
	lin := func(c float64) float64 {
		s := c / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	l := 0.2126*lin(float64(r)) + 0.7152*lin(float64(g)) + 0.0722*lin(float64(b))
	if l > 0.42 {
		return "#0b0c10"
	}
	return "#ffffff"
}

var hexRe = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

func parseHexColor(hex string) (r, g, b int, ok bool) {
	m := hexRe.FindStringSubmatch(hex)
	if m == nil {
		return 0, 0, 0, false
	}
	var n int
	if _, err := fmt.Sscanf(m[1], "%06x", &n); err != nil {
		return 0, 0, 0, false
	}
	return (n >> 16) & 255, (n >> 8) & 255, n & 255, true
}
