package artwork

import (
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
)

// A stored logo fits within maxLogoWidth x maxLogoHeight: TMDB's originals run to 12000 px wide
// (184 MB decoded), while clients draw a logo a few hundred points wide and Couch TV's screens
// at most 1600 px.
const (
	maxLogoWidth  = 2560
	maxLogoHeight = 1280
)

// fitLogo crops a logo PNG in place to the smallest box holding its visible pixels, and scales
// it down to fit within maxLogoWidth x maxLogoHeight. Clients draw a logo from the leading edge,
// beside text such as the title's year, so transparent padding around the wordmark would read
// as an indent; trimming it here also keeps the recorded size, and the aspect clients lay the
// logo out by, true to what shows.
func fitLogo(path string) error {
	img, err := decodePNG(path)
	if err != nil {
		return err
	}
	box := visibleBounds(img)
	if box.Empty() {
		return nil
	}
	w, h := fitSize(box.Dx(), box.Dy(), maxLogoWidth, maxLogoHeight)
	if box == img.Bounds() && w == box.Dx() && h == box.Dy() {
		return nil
	}
	out := img.SubImage(box).(*image.NRGBA)
	if w != box.Dx() || h != box.Dy() {
		out = shrink(out, w, h)
	}
	tmp := path + ".fit"
	if err := writePNG(tmp, out); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// decodePNG reads a PNG as non-premultiplied RGBA, the form a logo is trimmed and scaled in.
func decodePNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n, nil
	}
	n := image.NewNRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n, nil
}

func writePNG(path string, img image.Image) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(out, img); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// visibleBounds is the smallest rectangle holding every pixel that is not fully transparent;
// empty when there is none.
func visibleBounds(img *image.NRGBA) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[img.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			if row[4*x+3] == 0 {
				continue
			}
			minX, maxX = min(minX, b.Min.X+x), max(maxX, b.Min.X+x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if maxX < minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// fitSize is w x h scaled down to fit within maxW x maxH, keeping the aspect; a maxH of 0 leaves
// the height free. It never scales up.
func fitSize(w, h, maxW, maxH int) (int, int) {
	scale := 1.0
	if w > maxW {
		scale = float64(maxW) / float64(w)
	}
	if maxH > 0 && h > maxH {
		scale = min(scale, float64(maxH)/float64(h))
	}
	if scale == 1 {
		return w, h
	}
	return max(1, int(math.Round(float64(w)*scale))), max(1, int(math.Round(float64(h)*scale)))
}

// shrink scales src down to w x h, each pixel the average of the area it covers, weighted by
// alpha: an average of straight colours would mix in the colour of the transparent pixels
// around a wordmark (black, as often as not) and draw a dark fringe around light letters.
func shrink(src *image.NRGBA, w, h int) *image.NRGBA {
	b := src.Bounds()
	cols, rows := spans(b.Dx(), w), spans(b.Dy(), h)
	area := float64(b.Dx()) / float64(w) * float64(b.Dy()) / float64(h)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	acc := make([]float64, 4*w)
	for y, ry := range rows {
		clear(acc)
		for j, wy := range ry.weights {
			row := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+ry.first+j):]
			for x, cx := range cols {
				var r, g, bl, a float64
				for i, wx := range cx.weights {
					p := row[4*(cx.first+i):]
					pa := float64(p[3]) * wx
					r += float64(p[0]) * pa
					g += float64(p[1]) * pa
					bl += float64(p[2]) * pa
					a += pa
				}
				acc[4*x] += r * wy
				acc[4*x+1] += g * wy
				acc[4*x+2] += bl * wy
				acc[4*x+3] += a * wy
			}
		}
		out := dst.Pix[y*dst.Stride:]
		for x := range w {
			a := acc[4*x+3]
			if a == 0 {
				continue
			}
			out[4*x] = toByte(acc[4*x] / a)
			out[4*x+1] = toByte(acc[4*x+1] / a)
			out[4*x+2] = toByte(acc[4*x+2] / a)
			out[4*x+3] = toByte(a / area)
		}
	}
	return dst
}

// span is the run of source pixels one output pixel covers along an axis, with how much of
// each it covers.
type span struct {
	first   int
	weights []float64
}

// spans shares n source pixels out among m output pixels (m <= n), each output pixel an equal
// stretch of n/m of them.
func spans(n, m int) []span {
	out := make([]span, m)
	for k := range out {
		lo, hi := float64(k)*float64(n)/float64(m), float64(k+1)*float64(n)/float64(m)
		s := span{first: int(lo)}
		for i := s.first; i < min(n, int(math.Ceil(hi))); i++ {
			s.weights = append(s.weights, min(float64(i+1), hi)-max(float64(i), lo))
		}
		out[k] = s
	}
	return out
}

func toByte(v float64) uint8 {
	return uint8(min(255, max(0, math.Round(v))))
}
