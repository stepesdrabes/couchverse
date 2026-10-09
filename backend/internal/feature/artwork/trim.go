package artwork

import (
	"image"
	"image/draw"
	"image/png"
	"os"
)

// trimTransparentMargins crops a logo PNG in place to the smallest box holding its visible
// pixels. Clients draw a logo from the leading edge, beside text such as the title's year, so
// transparent padding around the wordmark would read as an indent; trimming it here also keeps
// the recorded size, and the aspect clients lay the logo out by, true to what shows.
func trimTransparentMargins(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	box := visibleBounds(img)
	if box.Empty() || box == img.Bounds() {
		return nil
	}
	cropped := image.NewNRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	draw.Draw(cropped, cropped.Bounds(), img, box.Min, draw.Src)

	tmp := path + ".trim"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := png.Encode(out, cropped); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// visibleBounds is the smallest rectangle holding every pixel that is not fully transparent;
// empty when there is none.
func visibleBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0 {
				continue
			}
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if maxX < minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}
