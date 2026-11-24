package stellar

import (
	"crypto/sha256"
	"fmt"
	"html"
	"strings"
	"sync"
)

// A gate is a second, shorter way to tell a key: six signs in order, taken
// from a catalogue every implementation shares, the way a gate is dialled.
// The signs do not look like the sky of the key: they lead to it, as
// coordinates do. Whoever shows a gate may add a seventh sign, the origin:
// the sky of its own key.
//
// It is an aid, like the name: six distinct signs out of 64 carry some 36
// bits, far fewer than the digest the sky stands for.
const (
	// GateCatalogue is the number of signs in the catalogue.
	GateCatalogue = 64
	// GateSigns is the number of signs that tell a key.
	GateSigns = 6
)

var (
	catalogue     [GateCatalogue]*Constellation
	catalogueOnce sync.Once
)

// Sign returns sign k of the catalogue, for 0 <= k < GateCatalogue: the
// constellation of SHA-256("stellar/sign/v1\0" ‖ k), k as one byte. Only its
// figure and its name are used.
func Sign(k int) *Constellation {
	catalogueOnce.Do(func() {
		for i := range catalogue {
			catalogue[i] = New(sha256.Sum256(append([]byte("stellar/sign/v1\x00"), byte(i))))
		}
	})
	return catalogue[k]
}

// Gate returns the six signs of a digest, as indexes in the catalogue. They
// are distinct, and their order matters. They are drawn from a stream of
// their own, SHAKE256("stellar/gate/v1\0" ‖ digest): a sign already taken is
// drawn again.
func Gate(digest [32]byte) [GateSigns]int {
	s := newStreamFor("stellar/gate/v1\x00", digest)
	var gate [GateSigns]int
	var taken [GateCatalogue]bool
	for i := 0; i < GateSigns; {
		k := s.uintn(GateCatalogue)
		if taken[k] {
			continue
		}
		taken[k] = true
		gate[i] = k
		i++
	}
	return gate
}

// GlyphSVG renders the figure alone, as a sign small enough to stand in a
// line: full strokes in the colour of the text around it, no night, no name.
func (c *Constellation) GlyphSVG() string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="60 20 880 880" role="img" aria-label="%s"><title>%s</title>`,
		html.EscapeString(c.Name), html.EscapeString(c.Name))
	b.WriteString(`<g fill="currentColor" stroke="currentColor" stroke-width="24" stroke-linecap="round">`)
	for _, e := range c.Edges {
		ax, ay := starXY(c.Stars[e.A])
		bx, by := starXY(c.Stars[e.B])
		fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f"/>`, ax, ay, bx, by)
	}
	for _, s := range c.Stars {
		x, y := starXY(s)
		fmt.Fprintf(&b, `<circle cx="%.0f" cy="%.0f" r="%.0f" stroke="none"/>`, x, y, magnitudeRadius[s.Magnitude]*1.7)
	}
	b.WriteString(`</g></svg>`)
	return b.String()
}
