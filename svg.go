package stellar

import (
	"fmt"
	"html"
	"strings"
)

// classColors are the glow and core colours of the spectral classes, from
// hot blue to cool red.
var classColors = [Classes]string{"#8fb4ff", "#f4f6ff", "#ffe08a", "#ffb070", "#ff7b6b"}

// magnitudeRadius is the core radius of a star, in viewBox units.
var magnitudeRadius = [Magnitudes]float64{11, 8, 5.5}

// SVG renders the constellation as a standalone SVG document, size pixels
// wide and high. The caption is drawn under the name; it is typically the
// textual fingerprint, and may be empty.
func (c *Constellation) SVG(size int, caption string) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000" width="%d" height="%d" role="img">`+"\n", size, size)
	w("<title>%s</title>\n", html.EscapeString(c.Name))
	w("<desc>Stellar v%d constellation of %s</desc>\n", Version, html.EscapeString(caption))

	w("<defs>\n")
	w(`<radialGradient id="sky" cx="50%%" cy="42%%" r="75%%"><stop offset="0" stop-color="#16204a"/><stop offset="1" stop-color="#05060f"/></radialGradient>` + "\n")
	for i, col := range classColors {
		w(`<radialGradient id="glow%d"><stop offset="0" stop-color="%s" stop-opacity=".55"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`+"\n", i, col, col)
	}
	w("</defs>\n")
	w(`<rect width="1000" height="1000" rx="36" fill="url(#sky)"/>` + "\n")

	w(`<g fill="#cfd8ff">` + "\n")
	for _, f := range c.Field {
		x := (float64(f.X) + 0.5) / FieldGrid * 1000
		y := (float64(f.Y) + 0.5) / FieldGrid * 1000
		w(`<circle cx="%.1f" cy="%.1f" r="%.1f" opacity="%.2f"/>`+"\n", x, y, 0.9+0.45*float64(f.Brightness), 0.25+0.17*float64(f.Brightness))
	}
	w("</g>\n")

	w(`<g stroke="#dfe6ff" stroke-opacity=".5" stroke-width="2.2" stroke-linecap="round">` + "\n")
	for _, e := range c.Edges {
		ax, ay := starXY(c.Stars[e.A])
		bx, by := starXY(c.Stars[e.B])
		w(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`+"\n", ax, ay, bx, by)
	}
	w("</g>\n")

	for _, s := range c.Stars {
		x, y := starXY(s)
		r := magnitudeRadius[s.Magnitude]
		col := classColors[s.Class]
		w(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#glow%d)"/>`+"\n", x, y, r*4, s.Class)
		if s.Magnitude == 0 {
			// Diffraction spikes single out the brightest stars.
			w(`<path d="M%.1f %.1fH%.1fM%.1f %.1fV%.1f" stroke="%s" stroke-opacity=".7" stroke-width="1.6" stroke-linecap="round"/>`+"\n",
				x-r*3.2, y, x+r*3.2, x, y-r*3.2, y+r*3.2, col)
		}
		w(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>`+"\n", x, y, r, col)
		w(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#fff"/>`+"\n", x, y, r*0.45)
	}

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
