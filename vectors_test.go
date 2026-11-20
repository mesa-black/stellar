package stellar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/v1.json from the current implementation")

// vector is one entry of testdata/v1.json: a digest and the constellation v1
// derives from it. The file is the reference for other implementations of
// the mapping; they must reproduce every entry exactly.
type vector struct {
	Digest string   `json:"digest"`
	Name   string   `json:"name"`
	Stars  [][4]int `json:"stars"` // x, y, magnitude, class
	Edges  [][2]int `json:"edges"` // a, b
	Field  [][3]int `json:"field"` // x, y, brightness
}

func vectorOf(digest [32]byte) vector {
	c := New(digest)
	v := vector{Digest: hex.EncodeToString(digest[:]), Name: c.Name}
	for _, s := range c.Stars {
		v.Stars = append(v.Stars, [4]int{s.X, s.Y, s.Magnitude, s.Class})
	}
	for _, e := range c.Edges {
		v.Edges = append(v.Edges, [2]int{e.A, e.B})
	}
	for _, f := range c.Field {
		v.Field = append(v.Field, [3]int{f.X, f.Y, f.Brightness})
	}
	return v
}

func vectorDigests() [][32]byte {
	var zero, ones [32]byte
	for i := range ones {
		ones[i] = 0xff
	}
	key, _ := Parse([]byte(testSSH))
	out := [][32]byte{zero, ones, key.Digest, sha256.Sum256(nil), sha256.Sum256([]byte("stellar"))}
	for i := 0; i < 27; i++ {
		out = append(out, digestOf(i))
	}
	return out
}

func TestVectors(t *testing.T) {
	const path = "testdata/v1.json"
	var want []vector
	for _, d := range vectorDigests() {
		want = append(want, vectorOf(d))
	}
	// One vector per line keeps the file diffable and small.
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, v := range want {
		line, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		if i < len(want)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if !bytes.Equal(got, buf.Bytes()) {
		t.Errorf("%s no longer matches the implementation: the v1 mapping has changed", path)
	}
}
