package stellar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// PNG renders the constellation as a picture w pixels wide and h high, for
// the places an SVG cannot go: the preview a messaging application shows
// under a link. It carries no text: the standard library draws none, and the
// name travels in the link's title.
//
// Like every rendering it is not normative, but it shows what a v1 renderer
// must: every main star with its position, magnitude and class, every edge.
func (c *Constellation) PNG(w, h int) []byte {
	cv := newCanvas(w, h)

	// Night: the same gradient as the SVG, centred a little above the middle.
	inner, outer := rgb(0x16, 0x20, 0x4a), rgb(0x05, 0x06, 0x0f)
	cx, cy, reach := float64(w)/2, float64(h)*0.42, math.Hypot(float64(w), float64(h))*0.55
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := math.Min(1, math.Hypot(float64(x)-cx, float64(y)-cy)/reach)
			cv.set(x, y, inner.mix(outer, t))
		}
	}

	// The background stars are spread over the whole picture.
	faint := rgb(0xcf, 0xd8, 0xff)
	for _, f := range c.Field {
		x := (float64(f.X) + 0.5) / FieldGrid * float64(w)
		y := (float64(f.Y) + 0.5) / FieldGrid * float64(h)
		cv.disc(x, y, 0.9+0.5*float64(f.Brightness), faint, 0.25+0.17*float64(f.Brightness))
	}

	// The constellation keeps its square, centred: the SVG's 1000-unit frame,
	// scaled so that the area the stars can occupy fills the height.
	scale := float64(h) / 900
	place := func(s Star) (float64, float64) {
		x, y := starXY(s)
		return float64(w)/2 + (x-500)*scale, float64(h)/2 + (y-460)*scale
	}

	line := rgb(0xdf, 0xe6, 0xff)
	for _, e := range c.Edges {
		ax, ay := place(c.Stars[e.A])
		bx, by := place(c.Stars[e.B])
		cv.segment(ax, ay, bx, by, 2.2*scale*1.4, line, 0.5)
	}
	for _, s := range c.Stars {
		x, y := place(s)
		r := magnitudeRadius[s.Magnitude] * scale * 1.4
		col := hexColor(classColors[s.Class])
		cv.glow(x, y, r*4, col, 0.55)
		if s.Magnitude == 0 {
			cv.segment(x-r*3.2, y, x+r*3.2, y, 1.6*scale*1.4, col, 0.7)
			cv.segment(x, y-r*3.2, x, y+r*3.2, 1.6*scale*1.4, col, 0.7)
		}
		cv.disc(x, y, r, col, 1)
		cv.disc(x, y, r*0.45, rgb(0xff, 0xff, 0xff), 1)
	}

	var out bytes.Buffer
	// The encoder only fails on an invalid image or a failing writer.
	_ = png.Encode(&out, cv.image())
	return out.Bytes()
}

// A small painter: straight alpha blending on a float buffer, with edges
// softened over one pixel. Enough for discs, glows and lines; nothing here is
// a general rasteriser.
type fcolor struct{ r, g, b float64 }

func rgb(r, g, b uint8) fcolor { return fcolor{float64(r), float64(g), float64(b)} }

func (a fcolor) mix(b fcolor, t float64) fcolor {
	return fcolor{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// hexColor reads "#rrggbb".
func hexColor(s string) fcolor {
	hex := func(c byte) uint8 {
		switch {
		case c >= '0' && c <= '9':
			return c - '0'
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10
		}
		return 0
	}
	return rgb(hex(s[1])<<4|hex(s[2]), hex(s[3])<<4|hex(s[4]), hex(s[5])<<4|hex(s[6]))
}

type canvas struct {
	w, h int
	px   []fcolor
}

func newCanvas(w, h int) *canvas { return &canvas{w, h, make([]fcolor, w*h)} }

func (cv *canvas) set(x, y int, c fcolor) { cv.px[y*cv.w+x] = c }

func (cv *canvas) blend(x, y int, c fcolor, alpha float64) {
	if x < 0 || y < 0 || x >= cv.w || y >= cv.h || alpha <= 0 {
		return
	}
	i := y*cv.w + x
	cv.px[i] = cv.px[i].mix(c, math.Min(1, alpha))
}

// each calls fn for every pixel of the box around a shape, with the pixel's
// centre.
func (cv *canvas) each(x0, y0, x1, y1 float64, fn func(x, y int, px, py float64)) {
	for y := int(math.Floor(y0)); y <= int(math.Ceil(y1)); y++ {
		for x := int(math.Floor(x0)); x <= int(math.Ceil(x1)); x++ {
			fn(x, y, float64(x)+0.5, float64(y)+0.5)
		}
	}
}

func coverage(edge float64) float64 { return math.Max(0, math.Min(1, edge+0.5)) }

func (cv *canvas) disc(cx, cy, r float64, c fcolor, alpha float64) {
	cv.each(cx-r-1, cy-r-1, cx+r+1, cy+r+1, func(x, y int, px, py float64) {
		cv.blend(x, y, c, alpha*coverage(r-math.Hypot(px-cx, py-cy)))
	})
}

// glow fades from alpha at the centre to nothing at radius r.
func (cv *canvas) glow(cx, cy, r float64, c fcolor, alpha float64) {
	cv.each(cx-r, cy-r, cx+r, cy+r, func(x, y int, px, py float64) {
		cv.blend(x, y, c, alpha*(1-math.Hypot(px-cx, py-cy)/r))
	})
}

func (cv *canvas) segment(ax, ay, bx, by, width float64, c fcolor, alpha float64) {
	dx, dy := bx-ax, by-ay
	length2 := dx*dx + dy*dy
	half := width / 2
	cv.each(math.Min(ax, bx)-half-1, math.Min(ay, by)-half-1, math.Max(ax, bx)+half+1, math.Max(ay, by)+half+1, func(x, y int, px, py float64) {
		t := 0.0
		if length2 > 0 {
			t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/length2))
		}
		cv.blend(x, y, c, alpha*coverage(half-math.Hypot(px-(ax+t*dx), py-(ay+t*dy))))
	})
}

func (cv *canvas) image() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, cv.w, cv.h))
	for i, p := range cv.px {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = clamp8(p.r), clamp8(p.g), clamp8(p.b), 0xff
	}
	return img
}

func clamp8(v float64) uint8 { return color.RGBA{R: uint8(math.Max(0, math.Min(255, v+0.5)))}.R }
