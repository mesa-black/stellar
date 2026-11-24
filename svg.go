package stellar

import (
	"fmt"
	"html"
	"strings"
)

// The figure is drawn as a glyph, in one ink: a hollow stroke from star to
// star, swelling into a round cell at each star. Nothing is told by colour.
const (
	glyphInk = "#e8ecff"
	// glyphStroke is the width of a stroke and glyphWall the thickness of its
	// outline, in viewBox units.
	glyphStroke = 15.0
	glyphWall   = 4.2
)

// magnitudeRadius is the radius of a star's cell, in viewBox units: the
// brighter the star, the larger the cell.
var magnitudeRadius = [Magnitudes]float64{23, 18, 13.5}

// SVG renders the constellation as a standalone SVG document, size pixels
// wide and high. The caption is drawn under the name; it is typically the
// textual fingerprint, and may be empty.
func (c *Constellation) SVG(size int, caption string) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000" width="%d" height="%d" role="img">`+"\n", size, size)
	w("<title>%s</title>\n", html.EscapeString(c.Name))
	w("<desc>Stellar v%d constellation of %s</desc>\n", Version, html.EscapeString(caption))

	// The night is painted in page coordinates, so that the inside of the
	// glyph can be painted with it again and look cut out.
	w("<defs>\n")
	w(`<radialGradient id="sky" gradientUnits="userSpaceOnUse" cx="500" cy="420" r="750"><stop offset="0" stop-color="#16204a"/><stop offset="1" stop-color="#05060f"/></radialGradient>` + "\n")
	w("</defs>\n")
	w(`<rect width="1000" height="1000" rx="4" fill="url(#sky)"/>` + "\n")

	w(`<g fill="#cfd8ff">` + "\n")
	for _, f := range c.Field {
		x := (float64(f.X) + 0.5) / FieldGrid * 1000
		y := (float64(f.Y) + 0.5) / FieldGrid * 1000
		w(`<circle cx="%.1f" cy="%.1f" r="%.1f" opacity="%.2f"/>`+"\n", x, y, 0.9+0.4*float64(f.Brightness), 0.14+0.1*float64(f.Brightness))
	}
	w("</g>\n")

	// The whole figure in ink, then its inside in night: what is left is the
	// outline of one shape, strokes and cells together.
	layer := func(paint string, inset float64) {
		w(`<g fill="%s" stroke="%s" stroke-width="%.1f" stroke-linecap="round">`+"\n", paint, paint, glyphStroke-2*inset)
		for _, e := range c.Edges {
			ax, ay := starXY(c.Stars[e.A])
			bx, by := starXY(c.Stars[e.B])
			w(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`+"\n", ax, ay, bx, by)
		}
		for _, s := range c.Stars {
			x, y := starXY(s)
			w(`<circle cx="%.1f" cy="%.1f" r="%.1f" stroke="none"/>`+"\n", x, y, magnitudeRadius[s.Magnitude]-inset)
		}
		w("</g>\n")
	}
	layer(glyphInk, 0)
	layer("url(#sky)", glyphWall)

	// The class of a star is the mark in its cell: none, a point, a full
	// cell, a bar, a cross.
	w(`<g fill="%s" stroke="%s" stroke-width="%.1f" stroke-linecap="round">`+"\n", glyphInk, glyphInk, glyphWall)
	for _, s := range c.Stars {
		x, y := starXY(s)
		r := magnitudeRadius[s.Magnitude] - glyphWall
		switch s.Class {
		case 1:
			w(`<circle cx="%.1f" cy="%.1f" r="%.1f" stroke="none"/>`+"\n", x, y, r*0.36)
		case 2:
			w(`<circle cx="%.1f" cy="%.1f" r="%.1f" stroke="none"/>`+"\n", x, y, r)
		case 3:
			w(`<path d="M%.1f %.1fH%.1f"/>`+"\n", x-r, y, x+r)
		case 4:
			w(`<path d="M%.1f %.1fH%.1fM%.1f %.1fV%.1f"/>`+"\n", x-r, y, x+r, x, y-r, y+r)
		}
	}
	w("</g>\n")

	w(`<text x="500" y="935" text-anchor="middle" font-family="ui-sans-serif, system-ui, sans-serif" font-size="34" letter-spacing="10" fill="#e8ecff">%s</text>`+"\n",
		html.EscapeString(strings.ToUpper(c.Name)))
	if caption != "" {
		w(`<text x="500" y="972" text-anchor="middle" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="17" fill="#e8ecff" fill-opacity=".6">%s</text>`+"\n",
			html.EscapeString(caption))
	}
	w("</svg>\n")
	return b.String()
}

// starXY maps a main star to viewBox coordinates, leaving room for the
// caption at the bottom.
func starXY(s Star) (x, y float64) {
	return 110 + (float64(s.X)+0.5)/StarGrid*780, 70 + (float64(s.Y)+0.5)/StarGrid*780
}
