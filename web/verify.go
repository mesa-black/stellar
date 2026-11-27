package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/mesa-black/stellar"
)

// Checking a key: a visitor who was given a key by another road, as a file,
// a line of text or a fingerprint, pastes it on the page of a host, and the
// site says whether it is the key that host serves. The answer comes from
// the two digests, compared whole. The six signs of the key given are shown
// beside it, each lit if the host has the same sign at that place: they are
// the picture of the answer, never the answer.

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

// verification is a key the visitor gave, compared with the key shown.
type verification struct {
	// Match is the answer: the two digests are equal.
	Match bool
	// Near is the rare case the signs cannot tell: six signs alike, and two
	// different keys.
	Near bool
	// Name and Fingerprint are those of the key given.
	Name, Fingerprint string
	Signs             []verifiedSign
	// Gate is the six signs of the key given, by their numbers in the
	// catalogue, for the script; Differ how many differ from the host's.
	Gate   string
	Differ int
	// When the keys differ, the public logs can say whether the key given
	// holds a certificate for this name all the same: a site may serve more
	// than one key. LogsURL offers to ask them, and is empty when they
	// cannot be read or were just asked; Logged is what they answered:
	// "yes", "no", "busy" or "failed".
	LogsURL string
	Logged  string
}

// verifiedSign is one sign of the key given, and whether the key shown has
// the same sign at that place.
type verifiedSign struct {
	sign
	Same bool
}

// check compares a key the visitor gave with the key the page shows.
func (v *view) check(given stellar.Key) {
	result := &verification{
		Match:       given.Digest == v.digest,
		Name:        stellar.New(given.Digest).Name,
		Fingerprint: given.Fingerprint(),
	}
	shown := stellar.Gate(v.digest)
	var numbers []string
	for i, k := range stellar.Gate(given.Digest) {
		g := stellar.Sign(k)
		same := k == shown[i]
		if !same {
			result.Differ++
		}
		numbers = append(numbers, strconv.Itoa(k))
		result.Signs = append(result.Signs, verifiedSign{
			sign: sign{Name: g.Name, Word: "Chevron " + strconv.Itoa(i+1), SVG: template.HTML(g.GlyphSVG())},
			Same: same,
		})
	}
	result.Gate = strings.Join(numbers, ",")
	result.Near = !result.Match && result.Differ == 0
	v.Verify, v.Signs = result, "/signs/"+signs().name
}

// inLogs settles, when asked, what two different keys mean. A site of some
// size serves several keys, and every certificate a public authority issues
// is written in public logs: a key given that is found there for this name
// is one of the site's own. One that is not there is what the certificate
// of somebody in the way looks like, since such a certificate is never
// logged.
//
// Asking sends the name of the host to the service that reads the logs, so
// nothing is asked until the visitor asks, as for the logs of the page.
func (s *Server) inLogs(r *http.Request, v *view, given stellar.Key) {
	if v.Verify.Match || s.Logs == nil {
		return
	}
	if r.FormValue("logs") != "1" {
		ask := url.Values{"host": {v.Host}, "key": {given.Fingerprint()}, "logs": {"1"}}
		if v.LangParam != "" {
			ask.Set("lang", v.LangParam)
		}
		v.Verify.LogsURL = "/verify?" + ask.Encode() + "#verify"
		return
	}
	issuances, err := s.logged(r.Context(), v.Host)
	switch {
	case errors.Is(err, errLogsBusy):
		v.Verify.Logged = "busy"
	case err != nil:
		v.Verify.Logged = "failed"
	default:
		v.Verify.Logged = "no"
		for _, issued := range issuances {
			if issued.Key == given.Digest {
				v.Verify.Logged = "yes"
				break
			}
		}
	}
}

// verify answers the form under a host's gate: the page of that host, with
// what came of comparing its key with the one pasted. A private key is
// refused before anything reads it.
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxKeyBytes)
	query := r.FormValue("host")
	v := s.newView(r, query)
	status := s.showHost(r, &v, query)
	if v.Host != "" {
		given := r.FormValue("key")
		if strings.Contains(given, "PRIVATE KEY") {
			v.VerifyError, status = v.S("verify.private"), http.StatusUnprocessableEntity
		} else if key, err := stellar.Parse([]byte(given)); err != nil {
			v.VerifyError, status = v.S("error.key"), http.StatusUnprocessableEntity
		} else {
			v.check(key)
			s.inLogs(r, &v, key)
		}
	}
	if v.Base != "" {
		v.CardHost = v.Host
	}
	v.finish()
	render(w, status, v)
}
