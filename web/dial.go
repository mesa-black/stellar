package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/mesa-black/stellar"
)

// Dialling a gate: the page lays out the 64 signs of the catalogue, and a
// visitor who was given six of them by another road picks them in order.
// Each one locks if it is the sign the key shown has at that place. It is
// the comparison the eye would make between two rows of signs, made by
// hand: it proves no more than the gate does.

// signsSide is how many signs the picture of the catalogue has in a row.
const signsSide = 8

// signs is the catalogue as one picture, the 64 signs in rows of eight, and
// the name it is served under, made from its contents: the picture may be
// kept for ever, and a change in how a sign is drawn changes the name.
var signs = sync.OnceValue(func() (sheet struct{ name, svg string }) {
	const cell = 880
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, signsSide*cell, signsSide*cell)
	for k := 0; k < stellar.GateCatalogue; k++ {
		glyph := stellar.Sign(k).GlyphSVG()
		// The drawing alone, without the frame a sign has when it stands by
		// itself, and in the ink of the page: a picture has no text colour
		// to inherit.
		from, to := strings.Index(glyph, "<g "), strings.LastIndex(glyph, "</svg>")
		fmt.Fprintf(&b, `<svg x="%d" y="%d" width="%d" height="%d" viewBox="60 20 880 880">%s</svg>`,
			k%signsSide*cell, k/signsSide*cell, cell, cell, strings.ReplaceAll(glyph[from:to], "currentColor", "#e8ecff"))
	}
	b.WriteString(`</svg>`)
	sheet.svg = b.String()
	sum := sha256.Sum256([]byte(sheet.svg))
	sheet.name = "signs-" + hex.EncodeToString(sum[:])[:12] + ".svg"
	return sheet
})

// signsFile serves the picture of the catalogue, /signs/<name>.
func signsFile(w http.ResponseWriter, r *http.Request) {
	sheet := signs()
	if r.PathValue("file") != sheet.name {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	io.WriteString(w, sheet.svg)
}

// catalogueSign is one sign of the catalogue on the page: its number and
// its name, said on hover.
type catalogueSign struct {
	K    int
	Name string
}

var catalogueSigns = sync.OnceValue(func() []catalogueSign {
	all := make([]catalogueSign, stellar.GateCatalogue)
	for k := range all {
		all[k] = catalogueSign{K: k, Name: stellar.Sign(k).Name}
	}
	return all
})

// dial fills what the page needs to let a gate be dialled: the catalogue,
// its picture, and the six signs expected, by their numbers.
func (v *view) dial(digest [32]byte) {
	var expected []string
	for _, k := range stellar.Gate(digest) {
		expected = append(expected, strconv.Itoa(k))
	}
	v.Dial, v.Catalogue, v.Signs = strings.Join(expected, ","), catalogueSigns(), "/signs/"+signs().name
}
