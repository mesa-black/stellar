package stellar

import (
	"bytes"
	"image/png"
	"math"
	"testing"
)

func TestPNG(t *testing.T) {
	c := New(digestOf(3))
	data := c.PNG(1200, 630)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1200 || b.Dy() != 630 {
		t.Fatalf("size %v", b)
	}
	if !bytes.Equal(data, New(digestOf(3)).PNG(1200, 630)) {
		t.Error("two renderings of the same digest differ")
	}
	if bytes.Equal(data, New(digestOf(4)).PNG(1200, 630)) {
		t.Error("two digests give the same picture")
	}

	// Every main star is on the picture: the wall of its cell is ink, which
	// no part of the night is. A stroke that enters the cell opens the wall
	// there, so the wall is looked for all around.
	scale := 630.0 / 900
	for i, s := range c.Stars {
		x, y := starXY(s)
		cx, cy := 600+(x-500)*scale, 315+(y-460)*scale
		r := (magnitudeRadius[s.Magnitude] - glyphWall/2) * scale * 1.25
		found := false
		for a := 0.0; a < 2*math.Pi && !found; a += 0.05 {
			r, g, b, _ := img.At(int(cx+r*math.Cos(a)), int(cy+r*math.Sin(a))).RGBA()
			found = r>>8 >= 0xd0 && g>>8 >= 0xd0 && b>>8 >= 0xd0
		}
		if !found {
			t.Errorf("star %d is not drawn: no ink around its centre", i)
		}
	}
	// And a corner is night.
	if r, g, b, _ := img.At(2, 628).RGBA(); r>>8 > 0x30 || g>>8 > 0x30 || b>>8 > 0x60 {
		t.Errorf("the corner is %02x%02x%02x", r>>8, g>>8, b>>8)
	}
}

func BenchmarkPNG(b *testing.B) {
	c := New(digestOf(3))
	for i := 0; i < b.N; i++ {
		c.PNG(1200, 630)
	}
}
