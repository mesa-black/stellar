package stellar

import (
	"crypto/sha256"
	"math"
	"testing"
)

func TestSignPlaces(t *testing.T) {
	for k := 0; k < GateCatalogue; k++ {
		p := SignPlace(k)
		if n := math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2]); math.Abs(n-1) > 1e-12 {
			t.Errorf("sign %d stands at distance %v from the centre", k, n)
		}
		for j := 0; j < k; j++ {
			q := SignPlace(j)
			if math.Hypot(math.Hypot(p[0]-q[0], p[1]-q[1]), p[2]-q[2]) < 0.2 {
				t.Errorf("signs %d and %d stand on each other", j, k)
			}
		}
	}
}

// The place is within the sphere, is the same each time, and has settled:
// it is the mean of the points nearest to it on its three lines.
func TestPlace(t *testing.T) {
	for i := 0; i < 5000; i++ {
		d := sha256.Sum256([]byte{byte(i), byte(i >> 8)})
		p := Place(d)
		n := math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2])
		if math.IsNaN(n) || n > 1 {
			t.Fatalf("digest %d is placed at %v", i, p)
		}
		if q := Place(d); q != p {
			t.Fatalf("digest %d has two places", i)
		}
		gate := Gate(d)
		var mean [3]float64
		for l := 0; l < GateSigns; l += 2 {
			a, b := SignPlace(gate[l]), SignPlace(gate[l+1])
			var along, length float64
			for j := range p {
				along += (p[j] - a[j]) * (b[j] - a[j])
				length += (b[j] - a[j]) * (b[j] - a[j])
			}
			at := math.Max(0, math.Min(1, along/length))
			for j := range p {
				mean[j] += (a[j] + at*(b[j]-a[j])) / 3
			}
		}
		for j := range p {
			if math.Abs(mean[j]-p[j]) > 1e-4 {
				t.Fatalf("digest %d has not settled: %v, mean of the nearest points %v", i, p, mean)
			}
		}
	}
}
