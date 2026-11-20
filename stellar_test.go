package stellar

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"
)

// A throwaway key generated for these tests; ssh-keygen -l prints testFP.
const (
	testSSH = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJJANI0eG+REEEvU37v4IluMHnioytjXEHI3blgM3+sZ test@stellar"
	testFP  = "SHA256:kqckL3tw6x8R1HqM3pohRjFi/6wO3yM2MXYGcV0/2qc"
)

func digestOf(i int) [32]byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(i))
	return sha256.Sum256(b[:])
}

func TestParseSSHMatchesSSHKeygen(t *testing.T) {
	for _, in := range []string{
		testSSH,
		"# a comment\n\n" + testSSH + "\n",
		`command="/bin/true",no-pty ` + testSSH,
	} {
		key, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if got := key.Fingerprint(); got != testFP {
			t.Errorf("Parse(%q) fingerprint = %s, want %s", in, got, testFP)
		}
		if key.Kind != "ssh-ed25519" || key.Label != "test@stellar" {
			t.Errorf("Parse(%q) = kind %q, label %q", in, key.Kind, key.Label)
		}
	}
}

func TestParseFingerprintRoundTrip(t *testing.T) {
	key, err := Parse([]byte(testFP + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := key.Fingerprint(); got != testFP {
		t.Errorf("fingerprint = %s, want %s", got, testFP)
	}
}

func TestParseRejects(t *testing.T) {
	for name, in := range map[string]string{
		"empty":             "",
		"garbage":           "hello world",
		"short fingerprint": "SHA256:abcd",
		"type mismatch":     strings.Replace(testSSH, "ssh-ed25519", "ssh-rsa", 1),
		"private key":       "-----BEGIN OPENSSH PRIVATE KEY-----\nAAAA\n-----END OPENSSH PRIVATE KEY-----\n",
	} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%s: Parse succeeded, want an error", name)
		}
	}
}

// TestGolden pins the v1 mapping: if it fails, the output for existing keys
// has changed and Version must be bumped.
func TestGolden(t *testing.T) {
	key, err := Parse([]byte(testSSH))
	if err != nil {
		t.Fatal(err)
	}
	c := New(key.Digest)
	if c.Name != "Subeve Zidu" {
		t.Errorf("name = %q, want %q", c.Name, "Subeve Zidu")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%v|%v|%v", c.Stars, c.Edges, c.Field)))
	const want = "0e120480ac629b16c7b76d31c6ff820b8d912644a70ba449903baea25c3a8f3e"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("structure hash = %s, want %s", got, want)
	}
}

func TestInvariants(t *testing.T) {
	for i := 0; i < 2000; i++ {
		c := New(digestOf(i))
		n := len(c.Stars)
		if n < MinStars || n > MaxStars {
			t.Fatalf("digest %d: %d stars", i, n)
		}
		for a := 0; a < n; a++ {
			for b := a + 1; b < n; b++ {
				if distSq(c.Stars[a], c.Stars[b]) < minDistSq {
					t.Fatalf("digest %d: stars %d and %d too close", i, a, b)
				}
			}
		}

		// Edges are distinct, well-formed, and connect every star.
		seen := map[Edge]bool{}
		parent := make([]int, n)
		for j := range parent {
			parent[j] = j
		}
		var find func(int) int
		find = func(j int) int {
			if parent[j] != j {
				parent[j] = find(parent[j])
			}
			return parent[j]
		}
		for _, e := range c.Edges {
			if e.A >= e.B || e.B >= n || seen[e] {
				t.Fatalf("digest %d: bad edge %v", i, e)
			}
			seen[e] = true
			parent[find(e.A)] = find(e.B)
		}
		for j := range parent {
			if find(j) != find(0) {
				t.Fatalf("digest %d: star %d is not connected", i, j)
			}
		}
		if len(c.Edges) < n-1 || len(c.Edges) > n+1 {
			t.Fatalf("digest %d: %d edges for %d stars", i, len(c.Edges), n)
		}
	}
}

func TestDeterministic(t *testing.T) {
	a, b := New(digestOf(7)), New(digestOf(7))
	if a.SVG(640, "x") != b.SVG(640, "x") || a.Text(true) != b.Text(true) {
		t.Error("two renderings of the same digest differ")
	}
}

// TestSingleBitFlip checks that the smallest change to a digest gives an
// unrelated constellation.
func TestSingleBitFlip(t *testing.T) {
	base := digestOf(1)
	ref := New(base)
	for bit := 0; bit < 256; bit++ {
		d := base
		d[bit/8] ^= 1 << (bit % 8)
		c := New(d)
		if c.Name == ref.Name {
			t.Errorf("bit %d: same name %q", bit, c.Name)
		}
		if fmt.Sprint(c.Stars) == fmt.Sprint(ref.Stars) {
			t.Errorf("bit %d: same stars", bit)
		}
	}
}

func TestNamesRarelyCollide(t *testing.T) {
	seen := map[string]int{}
	for i := 0; i < 5000; i++ {
		name := New(digestOf(i)).Name
		if j, ok := seen[name]; ok {
			// 5000 draws out of 80^5 names: a collision has odds of
			// about 1 in 260, low enough to flag.
			t.Errorf("digests %d and %d share the name %q", j, i, name)
		}
		seen[name] = i
	}
}

func TestSVGIsWellFormed(t *testing.T) {
	svg := New(digestOf(3)).SVG(640, `<caption & "quotes">`)
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("invalid SVG: %v", err)
		}
	}
	if strings.Contains(svg, `<caption`) {
		t.Error("caption is not escaped")
	}
}

func TestTextShape(t *testing.T) {
	for i := 0; i < 200; i++ {
		c := New(digestOf(i))
		lines := strings.Split(strings.TrimRight(c.Text(false), "\n"), "\n")
		if len(lines) != termRows+2 {
			t.Fatalf("digest %d: %d lines", i, len(lines))
		}
		glyphs := 0
		for _, l := range lines {
			if w := len([]rune(l)); w != termCols+2 {
				t.Fatalf("digest %d: line %q is %d columns wide", i, l, w)
			}
			glyphs += strings.Count(l, "✦") + strings.Count(l, "*") + strings.Count(l, "•")
		}
		if glyphs != len(c.Stars) {
			t.Fatalf("digest %d: %d glyphs for %d stars", i, glyphs, len(c.Stars))
		}
	}
}
