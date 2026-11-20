package stellar

import (
	"bytes"
	"image/png"
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

	// Every main star is on the picture: its centre is white, which no part
	// of the night is.
	scale := 630.0 / 900
	for i, s := range c.Stars {
		x, y := starXY(s)
		r, g, b, _ := img.At(int(600+(x-500)*scale), int(315+(y-460)*scale)).RGBA()
		if r>>8 < 0xf0 || g>>8 < 0xf0 || b>>8 < 0xf0 {
			t.Errorf("star %d is not drawn: %02x%02x%02x at its centre", i, r>>8, g>>8, b>>8)
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
