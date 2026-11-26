package web

import (
	"fmt"
	"html"
	"html/template"
	"math"
	"strings"

	"github.com/mesa-black/stellar"
)

// The chart is drawn once here, seen from one angle, so that it stands
// without a script. Every point also carries its place in space (data-p, or
// data-pts for a line), from which the script of the page turns it.
const (
	chartYaw   = 0.6
	chartPitch = 0.35
	// chartRadius is the sphere of the signs on the drawing, and chartEye how
	// far the eye stands from its centre, in radii.
	chartRadius = 100.0
	chartEye    = 5.0
	// chartAway is how far from the centre the origin stands, in radii: it
	// is not one of the places the signs lead to, and is drawn outside.
	chartAway = 1.38
)

// away pushes the place of the site that looked out of the sphere, in the
// direction it already had.
func away(p [3]float64) [3]float64 {
	n := math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2])
	if n == 0 {
		return [3]float64{0, chartAway, 0}
	}
	return [3]float64{p[0] / n * chartAway, p[1] / n * chartAway, p[2] / n * chartAway}
}

// project turns a place and returns where it falls on the drawing, and how
// near the eye it is: 1 at the front of the sphere, -1 at the back.
// app.js does the same.
func project(p [3]float64, yaw float64) (x, y, near float64) {
	cy, sy := math.Cos(yaw), math.Sin(yaw)
	cp, sp := math.Cos(chartPitch), math.Sin(chartPitch)
	px, pz := p[0]*cy+p[2]*sy, -p[0]*sy+p[2]*cy
	py, pz := p[1]*cp-pz*sp, p[1]*sp+pz*cp
	scale := chartRadius * chartEye / (chartEye - pz)
	return px * scale, -py * scale, pz
}

// nearest returns the point of the line from a to b nearest to p.
func nearest(p, a, b [3]float64) [3]float64 {
	var along, length float64
	for j := range p {
		along += (p[j] - a[j]) * (b[j] - a[j])
		length += (b[j] - a[j]) * (b[j] - a[j])
	}
	t := math.Max(0, math.Min(1, along/length))
	return [3]float64{a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1]), a[2] + t*(b[2]-a[2])}
}

// chart draws the place of a key among the signs: the sphere of the 64
// signs, the six of its gate joined two by two, the place they lead to, and,
// when the site knows its own key, the place it looked from, outside the
// sphere.
func chart(digest [32]byte, origin *[32]byte, arrival, from string) template.HTML {
	// Seen first from where the origin stands clear of the sphere, to its
	// right, rather than in front of it or behind.
	yaw := chartYaw
	var start [3]float64
	trip := origin != nil && *origin != digest
	if trip {
		start = away(stellar.Place(*origin))
		yaw = math.Atan2(start[2], start[0])
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-160 -160 320 320" role="img" aria-label="%s" data-yaw="%g" data-pitch="%g">`,
		html.EscapeString(arrival), yaw, chartPitch)
	fmt.Fprintf(&b, `<circle class="rim" r="%.1f"/>`, chartRadius*chartEye/math.Sqrt(chartEye*chartEye-1))

	line := func(class string, pts ...[3]float64) {
		var data, drawn []string
		for _, p := range pts {
			x, y, _ := project(p, yaw)
			data = append(data, fmt.Sprintf("%.4f %.4f %.4f", p[0], p[1], p[2]))
			drawn = append(drawn, fmt.Sprintf("%.1f,%.1f", x, y))
		}
		fmt.Fprintf(&b, `<polyline class="%s" data-pts="%s" points="%s"/>`, class, strings.Join(data, ";"), strings.Join(drawn, " "))
	}
	point := func(class string, p [3]float64, inside string) {
		x, y, near := project(p, yaw)
		fmt.Fprintf(&b, `<g class="%s" data-p="%.4f %.4f %.4f" transform="translate(%.1f %.1f)" opacity="%.2f">%s</g>`,
			class, p[0], p[1], p[2], x, y, 0.35+0.65*(near+1)/2, inside)
	}

	// Three circles of the sphere, to give it its roundness.
	for axis := 0; axis < 3; axis++ {
		var ring [][3]float64
		for i := 0; i <= 48; i++ {
			a := 2 * math.Pi * float64(i) / 48
			var p [3]float64
			p[(axis+1)%3], p[(axis+2)%3] = math.Cos(a), math.Sin(a)
			ring = append(ring, p)
		}
		line("ring", ring...)
	}

	gate := stellar.Gate(digest)
	chevron := map[int]int{}
	for i, k := range gate {
		chevron[k] = i + 1
	}
	here := stellar.Place(digest)
	// Each line, and what ties the arrival to it: a short stroke to the
	// point of the line nearest to it. The arrival is the middle of those
	// three points, which is all its definition says.
	var feet [][3]float64
	for i := 0; i < stellar.GateSigns; i += 2 {
		a, b := stellar.SignPlace(gate[i]), stellar.SignPlace(gate[i+1])
		line("chord", a, b)
		foot := nearest(here, a, b)
		line("tie", here, foot)
		feet = append(feet, foot)
	}
	if trip {
		line("trip", start, here)
	}
	for k := 0; k < stellar.GateCatalogue; k++ {
		if n, ok := chevron[k]; ok {
			point("lit", stellar.SignPlace(k), fmt.Sprintf(`<title>%s</title><circle r="2.6"/><text x="5" y="-4">%d</text>`,
				html.EscapeString(stellar.Sign(k).Name), n))
			continue
		}
		point("sign", stellar.SignPlace(k), `<circle r=".9"/>`)
	}
	for _, foot := range feet {
		point("foot", foot, `<circle r="1.6"/>`)
	}
	if trip {
		point("from", start, fmt.Sprintf(`<circle r="3.4"/><text y="13" text-anchor="middle">%s</text>`, html.EscapeString(from)))
	}
	point("arrival", here, fmt.Sprintf(`<circle r="3.4"/><circle class="halo" r="8"/><text x="12" y="3.5">%s</text>`, html.EscapeString(arrival)))
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
