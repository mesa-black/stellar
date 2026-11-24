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
	// Computed, not kept: a second copy of the picture to cut the inside of
	// the glyph out of the ink would double the memory a drawing takes.
	night := func(i int) fcolor {
		x, y := i%w, i/w
		return inner.mix(outer, math.Min(1, math.Hypot(float64(x)-cx, float64(y)-cy)/reach))
	}
	for i := range cv.px {
		cv.px[i] = night(i)
	}

	// The background stars are spread over the whole picture.
	faint := rgb(0xcf, 0xd8, 0xff)
	for _, f := range c.Field {
		x := (float64(f.X) + 0.5) / FieldGrid * float64(w)
		y := (float64(f.Y) + 0.5) / FieldGrid * float64(h)
		cv.disc(x, y, 0.9+0.45*float64(f.Brightness), flat(faint), 0.14+0.1*float64(f.Brightness))
	}

	// The constellation keeps its square, centred: the SVG's 1000-unit frame,
	// scaled so that the area the stars can occupy fills the height. Strokes
	// and cells are a little heavier than in the SVG: the picture is shown
	// small.
	scale := float64(h) / 900
	const heavy = 1.25
	place := func(s Star) (float64, float64) {
		x, y := starXY(s)
		return float64(w)/2 + (x-500)*scale, float64(h)/2 + (y-460)*scale
	}
	wall := glyphWall * scale * heavy

	// The whole figure in ink, then its inside in night, as in the SVG.
	ink := flat(hexColor(glyphInk))
	layer := func(p paint, inset float64) {
		for _, e := range c.Edges {
			ax, ay := place(c.Stars[e.A])
			bx, by := place(c.Stars[e.B])
			cv.segment(ax, ay, bx, by, glyphStroke*scale*heavy-2*inset, p, 1)
		}
		for _, s := range c.Stars {
			x, y := place(s)
			cv.disc(x, y, magnitudeRadius[s.Magnitude]*scale*heavy-inset, p, 1)
		}
	}
	layer(ink, 0)
	layer(night, wall)

	// The mark of the class, in the cell.
	for _, s := range c.Stars {
		x, y := place(s)
		r := magnitudeRadius[s.Magnitude]*scale*heavy - wall
		switch s.Class {
		case 1:
			cv.disc(x, y, r*0.36, ink, 1)
		case 2:
			cv.disc(x, y, r, ink, 1)
		case 3:
			cv.segment(x-r, y, x+r, y, wall, ink, 1)
		case 4:
			cv.segment(x-r, y, x+r, y, wall, ink, 1)
			cv.segment(x, y-r, x, y+r, wall, ink, 1)
		}
	}

	var out bytes.Buffer
	// The encoder only fails on an invalid image or a failing writer.
	_ = png.Encode(&out, cv.image())
	return out.Bytes()
}

// A small painter: straight alpha blending on a float buffer, with edges
// softened over one pixel. Enough for discs and lines; nothing here is a
// general rasteriser.
//
// A colour is three 32-bit floats: the buffer is the memory a drawing takes,
// twelve bytes a pixel, and the site draws within a small limit.
type fcolor struct{ r, g, b float32 }

func rgb(r, g, b uint8) fcolor { return fcolor{float32(r), float32(g), float32(b)} }

func (a fcolor) mix(b fcolor, t float64) fcolor {
	u := float32(t)
	return fcolor{a.r + (b.r-a.r)*u, a.g + (b.g-a.g)*u, a.b + (b.b-a.b)*u}
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

// paint gives the colour to lay on a pixel, from its index in the buffer.
type paint func(i int) fcolor

func flat(c fcolor) paint { return func(int) fcolor { return c } }

func (cv *canvas) blend(x, y int, p paint, alpha float64) {
	if x < 0 || y < 0 || x >= cv.w || y >= cv.h || alpha <= 0 {
		return
	}
	i := y*cv.w + x
	cv.px[i] = cv.px[i].mix(p(i), math.Min(1, alpha))
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

func (cv *canvas) disc(cx, cy, r float64, c paint, alpha float64) {
	cv.each(cx-r-1, cy-r-1, cx+r+1, cy+r+1, func(x, y int, px, py float64) {
		cv.blend(x, y, c, alpha*coverage(r-math.Hypot(px-cx, py-cy)))
	})
}

func (cv *canvas) segment(ax, ay, bx, by, width float64, c paint, alpha float64) {
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

func clamp8(v float32) uint8 {
	return color.RGBA{R: uint8(math.Max(0, math.Min(255, float64(v)+0.5)))}.R
}
