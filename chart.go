package stellar

import "math"

// A chart places a key in space, from its gate. It is a picture of the gate
// and nothing more: it carries no bit the six signs do not carry.
//
// The 64 signs of the catalogue stand at fixed places on a sphere of radius
// one. The six signs of a gate, taken two by two in their order (1-2, 3-4,
// 5-6), draw three lines through that sphere. Three lines in space seldom
// meet: the place of the key is the point nearest the three of them.

// SignPlace returns where sign k of the catalogue stands on the unit sphere:
// the signs are spread along a spiral from one pole to the other, each a
// golden angle further round than the one before.
func SignPlace(k int) [3]float64 {
	y := 1 - 2*(float64(k)+0.5)/GateCatalogue
	r := math.Sqrt(1 - y*y)
	phi := float64(k) * math.Pi * (3 - math.Sqrt(5))
	return [3]float64{r * math.Cos(phi), y, r * math.Sin(phi)}
}

// Place returns the place of a digest: the point that minimises the sum of
// its squared distances to the three lines of its gate, each line running
// from one sign to the other and no further. It lies within the sphere of
// the signs.
//
// Such a point is the mean of the points nearest to it on the three lines:
// it is found by starting from the middle of the lines and taking that mean
// again and again, which settles quickly.
func Place(digest [32]byte) [3]float64 {
	gate := Gate(digest)
	var a, b [GateSigns / 2][3]float64
	var p [3]float64
	for i := range a {
		a[i], b[i] = SignPlace(gate[2*i]), SignPlace(gate[2*i+1])
		for j := range p {
			p[j] += (a[i][j] + b[i][j]) / 2 / float64(len(a))
		}
	}
	for step := 0; step < 1000; step++ {
		var next [3]float64
		for i := range a {
			// The point of the line nearest to p.
			var along, length float64
			for j := range p {
				d := b[i][j] - a[i][j]
				along += (p[j] - a[i][j]) * d
				length += d * d
			}
			t := math.Max(0, math.Min(1, along/length))
			for j := range p {
				next[j] += (a[i][j] + t*(b[i][j]-a[i][j])) / float64(len(a))
			}
		}
		p = next
	}
	return p
}
