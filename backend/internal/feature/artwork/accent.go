package artwork

import (
	"fmt"
	"image"
	_ "image/jpeg" // register decoders for image.Decode
	_ "image/png"
	"math"
	"os"
)

// ExtractAccent decodes an image file and returns a vibrant accent colour as a
// hex string (e.g. "#3a7bd5"), or "" if the image can't be decoded (a webp
// upload - stdlib decodes jpeg/png only) or has no usable colour. Best effort:
// callers store the result and fall back to the theme accent on "".
func ExtractAccent(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return ""
	}
	return vibrantAccent(img)
}

// vibrantAccent samples the image into a coarse colour histogram and picks the
// most prominent saturated, mid-light colour - the same intent as the browser's
// node-vibrant "Vibrant" swatch. Falls back to the dominant colour when nothing
// is vibrant, so a muted image still themes to something sensible.
func vibrantAccent(img image.Image) string {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return ""
	}
	// sample a ~64x64 grid regardless of the source resolution
	const grid = 64
	stepX := max(1, w/grid)
	stepY := max(1, h/grid)

	type bucket struct {
		count   int
		r, g, b int
	}
	// 4 bits per channel -> at most 4096 buckets
	buckets := map[uint16]*bucket{}
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8000 {
				continue // mostly transparent
			}
			r, g, bl := int(r16>>8), int(g16>>8), int(b16>>8)
			key := uint16((r>>4)<<8 | (g>>4)<<4 | (bl >> 4))
			bk := buckets[key]
			if bk == nil {
				bk = &bucket{}
				buckets[key] = bk
			}
			bk.count++
			bk.r += r
			bk.g += g
			bk.b += bl
		}
	}
	if len(buckets) == 0 {
		return ""
	}

	best := -1.0
	var br, bg, bb int
	var domR, domG, domB, domCount int
	for _, bk := range buckets {
		r, g, bl := bk.r/bk.count, bk.g/bk.count, bk.b/bk.count
		if bk.count > domCount {
			domCount, domR, domG, domB = bk.count, r, g, bl
		}
		s, l := satLum(r, g, bl)
		if s < 0.25 || l < 0.15 || l > 0.85 {
			continue // skip near-grey and near-black/white
		}
		// favour saturated, mid-light and populous colours
		lightBell := 1 - math.Abs(l-0.5)/0.5
		score := float64(bk.count) * s * s * lightBell
		if score > best {
			best, br, bg, bb = score, r, g, bl
		}
	}
	if best < 0 {
		br, bg, bb = domR, domG, domB // nothing vibrant - theme to the dominant colour
	}
	return fmt.Sprintf("#%02x%02x%02x", br, bg, bb)
}

// satLum returns the HSL saturation and lightness of an 8-bit RGB colour.
func satLum(r, g, b int) (sat, lum float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	maxc := math.Max(rf, math.Max(gf, bf))
	minc := math.Min(rf, math.Min(gf, bf))
	lum = (maxc + minc) / 2
	d := maxc - minc
	if d == 0 {
		return 0, lum
	}
	if lum > 0.5 {
		return d / (2 - maxc - minc), lum
	}
	return d / (maxc + minc), lum
}
