// Package stellar turns a public-key fingerprint into a constellation.
//
// It is a visual fingerprint, in the spirit of OpenSSH randomart: two people
// (or one person at two moments) can compare a small sky instead of a long
// string of characters. It is not a cipher and adds no security of its own.
// The security rests entirely on the SHA-256 digest of the key it draws.
//
// The mapping from digest to constellation is deterministic and versioned.
// See docs/spec.md for the normative description.
package stellar

import (
	"crypto/sha3"
	"sort"
)

// Version is the version of the digest-to-constellation mapping. Any change
// that alters the output for a given digest must bump it.
const Version = 1

const (
	// StarGrid is the side of the integer grid main stars are placed on.
	StarGrid = 64
	// FieldGrid is the side of the integer grid background stars are placed on.
	FieldGrid = 256
	// MinStars and MaxStars bound the number of main stars.
	MinStars = 7
	MaxStars = 12
	// minDistSq is the minimum squared distance, in grid units, between two
	// main stars. At most 11 discs of radius 9 exclude fewer cells than the
	// grid holds, so placement by rejection always terminates.
	minDistSq = 9 * 9
	// Magnitudes and Classes are the number of brightness levels and of
	// spectral classes (colours) a main star can take.
	Magnitudes = 3
	Classes    = 5
)

// Star is a main star of the constellation.
type Star struct {
	// X and Y are coordinates on the StarGrid, origin at the top left.
	X, Y int
	// Magnitude runs from 0 (brightest) to Magnitudes-1 (faintest).
	Magnitude int
	// Class is the spectral class, an index in [0, Classes).
	Class int
}

// FieldStar is a faint background star.
type FieldStar struct {
	// X and Y are coordinates on the FieldGrid, origin at the top left.
	X, Y int
	// Brightness runs from 0 (faintest) to 2 (brightest).
	Brightness int
}

// Edge joins two main stars, identified by their index. A is always less
// than B.
type Edge struct{ A, B int }

// Constellation is the drawing derived from a digest.
type Constellation struct {
	Digest [32]byte
	// Name is an invented, Latin-sounding two-word name, meant to be read
	// aloud. It is not the name of a real constellation.
	Name  string
	Stars []Star
	Edges []Edge
	Field []FieldStar
}

// New derives the constellation of a SHA-256 digest.
func New(digest [32]byte) *Constellation {
	s := newStream(digest)
	c := &Constellation{Digest: digest}

	c.Name = name(s)

	n := MinStars + s.uintn(MaxStars-MinStars+1)
	for len(c.Stars) < n {
		x, y := s.uintn(StarGrid), s.uintn(StarGrid)
		if tooClose(c.Stars, x, y) {
			continue
		}
		c.Stars = append(c.Stars, Star{
			X:         x,
			Y:         y,
			Magnitude: s.uintn(Magnitudes),
			Class:     s.uintn(Classes),
		})
	}

	// Spanning tree: each star joins one of the two nearest stars placed
	// before it.
	adj := newAdjacency(n)
	for i := 1; i < n; i++ {
		near := nearest(c.Stars, i, func(j int) bool { return j < i })
		pick := 0
		if len(near) > 1 {
			pick = s.uintn(2)
		}
		c.Edges = append(c.Edges, adj.link(near[pick], i))
	}

	// Up to two extra edges close loops.
	for extra := s.uintn(3); extra > 0; extra-- {
		a := s.uintn(n)
		near := nearest(c.Stars, a, func(j int) bool { return j != a && !adj.has(a, j) })
		if len(near) == 0 {
			continue
		}
		c.Edges = append(c.Edges, adj.link(a, near[0]))
	}

	field := 40 + s.uintn(24)
	for i := 0; i < field; i++ {
		c.Field = append(c.Field, FieldStar{
			X:          s.uintn(FieldGrid),
			Y:          s.uintn(FieldGrid),
			Brightness: s.uintn(3),
		})
	}
	return c
}

// stream is a deterministic source of uniform integers, keyed by the digest.
type stream struct{ xof *sha3.SHAKE }

func newStream(digest [32]byte) *stream { return newStreamFor("stellar/v1\x00", digest) }

// newStreamFor starts the stream of one purpose, named by its domain.
func newStreamFor(domain string, digest [32]byte) *stream {
	xof := sha3.NewSHAKE256()
	xof.Write([]byte(domain))
	xof.Write(digest[:])
	return &stream{xof}
}

// uintn returns a uniform integer in [0, n), for 1 <= n <= 65536. It reads
// big-endian 16-bit words and rejects those that would bias the result.
func (s *stream) uintn(n int) int {
	limit := 65536 - 65536%n
	var b [2]byte
	for {
		s.xof.Read(b[:])
		if v := int(b[0])<<8 | int(b[1]); v < limit {
			return v % n
		}
	}
}

const (
	consonants = "bdfghjklmnprstvz"
	vowels     = "aeiou"
)

// The endings give the name its Latin sound: the first word reads as a noun,
// the second as an epithet, as in the names astronomers give constellations.
var (
	nounEndings    = [8]string{"a", "us", "um", "is", "ix", "or", "ax", "ia"}
	epithetEndings = [8]string{"alis", "aris", "ensis", "inus", "ianus", "osus", "atus", "icus"}
)

// name draws an invented, Latin-sounding name of two words, such as
// "Merebus Vovalis": a noun (consonant, vowel, consonant, vowel, consonant,
// ending) and an epithet (consonant, vowel, consonant, ending). There are
// about 8.4 billion names, or 33 bits. None is the name of a real
// constellation, except by accident.
func name(s *stream) string {
	consonant := func() byte { return consonants[s.uintn(len(consonants))] }
	vowel := func() byte { return vowels[s.uintn(len(vowels))] }
	upper := func(c byte) byte { return c - 'a' + 'A' }

	noun := []byte{upper(consonant()), vowel(), consonant(), vowel(), consonant()}
	noun = append(noun, nounEndings[s.uintn(len(nounEndings))]...)

	epithet := []byte{upper(consonant()), vowel(), consonant()}
	epithet = append(epithet, epithetEndings[s.uintn(len(epithetEndings))]...)

	return string(noun) + " " + string(epithet)
}

func distSq(a, b Star) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
}

func tooClose(stars []Star, x, y int) bool {
	p := Star{X: x, Y: y}
	for _, s := range stars {
		if distSq(s, p) < minDistSq {
			return true
		}
	}
	return false
}

// nearest returns the indices of the stars accepted by keep, ordered by
// increasing distance to star i, ties broken by index.
func nearest(stars []Star, i int, keep func(j int) bool) []int {
	var out []int
	for j := range stars {
		if keep(j) {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		da, db := distSq(stars[i], stars[out[a]]), distSq(stars[i], stars[out[b]])
		if da != db {
			return da < db
		}
		return out[a] < out[b]
	})
	return out
}

type adjacency [][]bool

func newAdjacency(n int) adjacency {
	adj := make(adjacency, n)
	for i := range adj {
		adj[i] = make([]bool, n)
	}
	return adj
}

func (adj adjacency) has(a, b int) bool { return adj[a][b] }

func (adj adjacency) link(a, b int) Edge {
	adj[a][b], adj[b][a] = true, true
	if a > b {
		a, b = b, a
	}
	return Edge{a, b}
}
