package stellar

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
)

// gateFile is testdata/gate-v1.json: the names of the catalogue, in order,
// and the gates of the digests testdata/v1.json lists. Sign k is the v1
// constellation of SHA-256("stellar/sign/v1\0" ‖ k), so its name is enough
// to tell that the catalogue is the right one.
type gateFile struct {
	Catalogue []string    `json:"catalogue"`
	Gates     []gateEntry `json:"gates"`
}

type gateEntry struct {
	Digest string `json:"digest"`
	Signs  []int  `json:"signs"`
}

func TestGateVectors(t *testing.T) {
	const path = "testdata/gate-v1.json"
	var want gateFile
	for k := 0; k < GateCatalogue; k++ {
		want.Catalogue = append(want.Catalogue, Sign(k).Name)
	}
	for _, d := range vectorDigests() {
		g := Gate(d)
		want.Gates = append(want.Gates, gateEntry{hex.EncodeToString(d[:]), g[:]})
	}
	data, err := json.MarshalIndent(want, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if *update {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("%s no longer matches the implementation: the catalogue or the gates have changed", path)
	}
}

func TestGateInvariants(t *testing.T) {
	names := map[string]int{}
	for k := 0; k < GateCatalogue; k++ {
		name := Sign(k).Name
		if j, dup := names[name]; dup {
			t.Errorf("signs %d and %d share the name %q", j, k, name)
		}
		names[name] = k
	}
	for i := 0; i < 2000; i++ {
		g := Gate(digestOf(i))
		if g != Gate(digestOf(i)) {
			t.Fatalf("digest %d: two gates for one digest", i)
		}
		seen := map[int]bool{}
		for _, k := range g {
			if k < 0 || k >= GateCatalogue || seen[k] {
				t.Fatalf("digest %d: gate %v", i, g)
			}
			seen[k] = true
		}
	}
	if Gate(digestOf(1)) == Gate(digestOf(2)) {
		t.Error("two digests share a gate")
	}
}

func TestGlyphSVGIsWellFormed(t *testing.T) {
	svg := Sign(0).GlyphSVG()
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("invalid SVG: %v", err)
		}
	}
	if !strings.Contains(svg, Sign(0).Name) || strings.Count(svg, "<line") != len(Sign(0).Edges) {
		t.Errorf("the sign does not show its figure and its name:\n%s", svg)
	}
}
