package web

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mesa-black/stellar"
)

func fakeLookup(calls *atomic.Int32) Lookup {
	return func(_ context.Context, host string) (stellar.Host, error) {
		calls.Add(1)
		if host == "down.example" {
			return stellar.Host{}, errors.New("dial tcp 10.1.2.3:443: connection refused")
		}
		return stellar.Host{
			Key:          stellar.Key{Digest: sha256.Sum256([]byte(host)), Kind: "X.509 certificate"},
			Version:      "TLS 1.3",
			KeyExchange:  "X25519MLKEM768",
			PostQuantum:  host != "classical.example",
			KeyAlgorithm: "ECDSA P-256",
			Issuer:       "Example CA",
			NotAfter:     time.Date(2027, 2, 18, 1, 36, 57, 0, time.UTC),
			Announcement: announcementOf(host),
		}, nil
	}
}

// announcementOf gives each test host what its DNS would say: its own key,
// another one, records Stellar does not read, nothing, or no answer at all.
func announcementOf(host string) *stellar.Announcement {
	switch host {
	case "announced.example":
		return &stellar.Announcement{Keys: [][32]byte{sha256.Sum256([]byte("old.example")), sha256.Sum256([]byte(host))}}
	case "signed.example":
		return &stellar.Announcement{Keys: [][32]byte{sha256.Sum256([]byte(host))}, Signed: true}
	case "stale.example":
		return &stellar.Announcement{Keys: [][32]byte{sha256.Sum256([]byte("old.example"))}}
	case "unread.example":
		return &stellar.Announcement{Other: 2}
	case "deaf.example":
		return nil
	}
	return &stellar.Announcement{}
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
	return rec
}

func TestCleanHost(t *testing.T) {
	good := map[string]string{
		"example.com":                   "example.com",
		"  Example.COM. ":               "example.com",
		"https://example.com/a/b?c=d":   "example.com",
		"https://example.com:8443/path": "example.com",
		"example.com/path":              "example.com",
		"xn--bcher-kva.example":         "xn--bcher-kva.example",
	}
	for in, want := range good {
		if got, err := cleanHost(in); err != nil || got != want {
			t.Errorf("cleanHost(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", "localhost", "127.0.0.1", "10.0.0.1", "[::1]", "::1", "example.com:22",
		"exa mple.com", "-a.example.com", "a..example.com", "user@example.com",
		"example.com\r\nHost: x", "192.168.1", "0x7f.1", "1.2.3.4.5", "bücher.example",
		strings.Repeat("a", 64) + ".example.com",
	} {
		if got, err := cleanHost(in); err == nil {
			t.Errorf("cleanHost(%q) = %q, want an error", in, got)
		}
	}
}

func TestIsPublic(t *testing.T) {
	for _, ip := range []string{"93.184.215.14", "2606:2800:21f:cb07:6820:80da:af6b:8b2c", "1.1.1.1"} {
		if !isPublic(netip.MustParseAddr(ip)) {
			t.Errorf("%s should be public", ip)
		}
	}
	for _, ip := range []string{
		"127.0.0.1", "10.0.0.1", "172.16.5.4", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255", "198.18.0.1",
		"::1", "::", "fe80::1", "fc00::1", "fd12:3456::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "64:ff9b::a00:1", "2002:a00:1::1",
	} {
		if isPublic(netip.MustParseAddr(ip)) {
			t.Errorf("%s should not be public", ip)
		}
	}
}

func TestPublicOnlyControl(t *testing.T) {
	if err := publicOnly("tcp4", "1.1.1.1:443", nil); err != nil {
		t.Errorf("public address refused: %v", err)
	}
	for _, addr := range []string{"127.0.0.1:443", "[::1]:443", "169.254.169.254:443", "nonsense"} {
		if err := publicOnly("tcp", addr, nil); !errors.Is(err, errNotPublic) {
			t.Errorf("publicOnly(%q) = %v, want errNotPublic", addr, err)
		}
	}
}

// TestPublicLookupRefusesLoopback goes through the real dialer: a listener on
// the loopback interface must never be reached.
func TestPublicLookupRefusesLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback listener:", err)
	}
	defer ln.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		if c, err := ln.Accept(); err == nil {
			accepted <- struct{}{}
			c.Close()
		}
	}()

	dialer := &net.Dialer{Control: publicOnly}
	_, err = stellar.FromHostWith(context.Background(), dialer, ln.Addr().String(), true)
	if !errors.Is(err, errNotPublic) {
		t.Fatalf("err = %v, want errNotPublic", err)
	}
	select {
	case <-accepted:
		t.Fatal("the loopback listener was reached")
	default:
	}
}

func TestHomeEmpty(t *testing.T) {
	var calls atomic.Int32
	rec := get(t, New(fakeLookup(&calls)).Handler(), "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Every key has its stargate") {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
	// The title's star is an SVG too; a constellation is the one with a role.
	if strings.Contains(rec.Body.String(), `role="img"`) {
		t.Error("the empty page draws a constellation")
	}
	if !strings.Contains(rec.Body.String(), "The names are invented") {
		t.Error("the empty page does not say the names are invented")
	}
	if !strings.Contains(rec.Body.String(), `class="star"`) {
		t.Error("the title has lost its star")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "font-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
	// Both leave the site, so both open in another window.
	for _, link := range []string{
		`href="https://github.com/mesa-black/sablier" rel="noopener noreferrer" target="_blank">Sablier<`,
		`href="https://mesa.black" rel="noopener noreferrer" target="_blank">BlackMesa Labs<`,
	} {
		if !strings.Contains(rec.Body.String(), link) {
			t.Errorf("the footer lacks %s", link)
		}
	}
}

func TestHomeShowsHost(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	rec := get(t, h, "/?host="+url.QueryEscape("https://Example.com/login"))
	body := rec.Body.String()
	want := stellar.New(sha256.Sum256([]byte("example.com")))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	for _, s := range []string{"<svg", want.Name, `data-host="example.com"`, "/sky/example.com.svg"} {
		if !strings.Contains(body, s) {
			t.Errorf("body lacks %q", s)
		}
	}

	// A second visit is served from the cache.
	get(t, h, "/?host=example.com")
	if n := calls.Load(); n != 1 {
		t.Errorf("%d lookups, want 1", n)
	}
}

func TestShare(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	h := s.Handler()
	name := stellar.New(sha256.Sum256([]byte("example.com"))).Name

	shared := func(body string) string {
		t.Helper()
		_, rest, ok := strings.Cut(body, `href="https://threema.id/compose?text=`)
		if !ok {
			t.Fatal("no Threema link")
		}
		raw, _, _ := strings.Cut(rest, `"`)
		text, err := url.QueryUnescape(strings.ReplaceAll(raw, "&amp;", "&"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(raw, "+") {
			t.Error("spaces are encoded as +")
		}
		return text
	}

	// Without a public address, the message names the sky and carries no link.
	text := shared(get(t, h, "/?host=example.com").Body.String())
	if !strings.Contains(text, "example.com") || !strings.Contains(text, name) || !strings.Contains(text, "SHA256:") {
		t.Errorf("message = %q", text)
	}
	if strings.Contains(text, "http") {
		t.Errorf("message links somewhere without a public address: %q", text)
	}

	s.BaseURL = "https://stellar.example"
	text = shared(get(t, h, "/?host=example.com").Body.String())
	if !strings.HasSuffix(text, "https://stellar.example/?host=example.com") {
		t.Errorf("message = %q", text)
	}

	if strings.Contains(get(t, h, "/").Body.String(), "threema") {
		t.Error("the empty page offers to share")
	}
}

// A fingerprint typed in the website field draws its key, fetches nothing and
// is not remembered as a host.
func TestHomeFingerprint(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	rec := get(t, h, "/?host="+url.QueryEscape(" SHA256:kqckL3tw6x8R1HqM3pohRjFi/6wO3yM2MXYGcV0/2qc "))
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "Subevia Zidensis") {
		t.Fatalf("status %d", rec.Code)
	}
	if calls.Load() != 0 {
		t.Error("a fingerprint triggered a lookup")
	}
	if !strings.Contains(body, `data-host=""`) {
		t.Error("a fingerprint is remembered as a host")
	}
	if rec := get(t, h, "/?host=SHA256:tooshort"); rec.Code != 422 {
		t.Errorf("bad fingerprint: status %d", rec.Code)
	}
}

// The empty page shows the sky of the site's own key, once it is known, and
// never waits for it.
func TestHomeSelfSky(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	h := s.Handler()
	if strings.Contains(get(t, h, "/").Body.String(), `class="self"`) {
		t.Error("a site that does not know its address shows a sky of its own")
	}

	s.BaseURL = "https://stellar.example"
	s.RefreshSelf()
	body := get(t, h, "/").Body.String()
	name := stellar.New(sha256.Sum256([]byte("stellar.example"))).Name
	for _, want := range []string{`class="self"`, "<svg", name, `href="/?host=stellar.example"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the empty page lacks %q", want)
		}
	}
	if strings.Contains(get(t, h, "/?host=example.com").Body.String(), `class="self"`) {
		t.Error("a result page also shows the site's own sky")
	}

	// A site whose own key cannot be fetched still serves its page.
	down := New(fakeLookup(&calls))
	down.BaseURL = "https://down.example"
	down.RefreshSelf()
	if rec := get(t, down.Handler(), "/"); rec.Code != 200 || strings.Contains(rec.Body.String(), `class="self"`) {
		t.Errorf("status %d", rec.Code)
	}
}

// A host's page says what its certificate and its handshake say, and whether
// the key exchange holds against a quantum computer. A pasted key has neither.
func TestHostFacts(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()

	body := get(t, h, "/?host=example.com").Body.String()
	for _, want := range []string{
		"ECDSA P-256, valid until 18 February 2027, issued by Example CA",
		"TLS 1.3, key exchange X25519MLKEM768",
		`data-pq="yes"`, "Post-quantum key exchange: yes.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	body = get(t, h, "/?host=classical.example").Body.String()
	if !strings.Contains(body, `data-pq="no"`) || !strings.Contains(body, "Post-quantum key exchange: no.") {
		t.Error("a classical key exchange is not said to be one")
	}
	body = get(t, h, "/?host="+url.QueryEscape("SHA256:kqckL3tw6x8R1HqM3pohRjFi/6wO3yM2MXYGcV0/2qc")).Body.String()
	if strings.Contains(body, "Post-quantum") || strings.Contains(body, "<dt>Connection</dt>") {
		t.Error("a fingerprint has no connection to describe")
	}
}

// The three catalogues say the same things: same keys, same placeholders.
func TestLanguagesAgree(t *testing.T) {
	holes := regexp.MustCompile(`%[sd]|\{[a-z]+\}|</?[a-z]+>`)
	for _, lang := range languages[1:] {
		for key, en := range messages["en"] {
			other, ok := messages[lang][key]
			if !ok {
				t.Errorf("%s lacks %q", lang, key)
				continue
			}
			want, got := holes.FindAllString(en, -1), holes.FindAllString(other, -1)
			sort.Strings(want)
			sort.Strings(got)
			if strings.Join(want, " ") != strings.Join(got, " ") {
				t.Errorf("%s %q: placeholders %v, English has %v", lang, key, got, want)
			}
		}
		for key := range messages[lang] {
			if _, ok := messages["en"][key]; !ok {
				t.Errorf("%s has %q, which English lacks", lang, key)
			}
		}
	}
}

// The language is the visitor's explicit choice, else the browser's, else
// English; an explicit choice is carried by the links and the forms.
func TestLanguage(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	page := func(target, accept string) string {
		req := httptest.NewRequest("GET", target, nil)
		if accept != "" {
			req.Header.Set("Accept-Language", accept)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Header().Get("Content-Language") + "|" + rec.Body.String()
	}

	if body := page("/", ""); !strings.HasPrefix(body, "en|") || !strings.Contains(body, "Every key has its stargate.") {
		t.Error("no preference is not English")
	}
	if body := page("/", "fr-FR,fr;q=0.9,en;q=0.8"); !strings.HasPrefix(body, "fr|") || !strings.Contains(body, "À chaque clé sa porte des étoiles.") || strings.Contains(body, `name="lang"`) {
		t.Error("a French browser is not answered in French, or the choice is forced on it")
	}
	if body := page("/", "de,es;q=0.5"); !strings.HasPrefix(body, "es|") {
		t.Error("the first language we speak is not the one chosen")
	}
	body := page("/?lang=es&host=example.com", "fr")
	for _, want := range []string{
		`<html lang="es">`, "Compartir este cielo", "18 de febrero de 2027", "certificado X.509",
		`<input type="hidden" name="lang" value="es">`, `href="/?host=example.com&amp;lang=fr"`,
		"El+cielo+de+su+clave", `data-same="El mismo cielo`,
	} {
		if !strings.Contains(strings.ReplaceAll(body, "%20", "+"), want) {
			t.Errorf("the Spanish page lacks %q", want)
		}
	}
	if body := page("/?lang=xx", ""); !strings.HasPrefix(body, "en|") {
		t.Error("an unknown language is not ignored")
	}
	if body := page("/?lang=fr&host=127.0.0.1", ""); !strings.Contains(body, "Saisissez un nom de site public") {
		t.Error("an error is not said in the page's language")
	}
}

// What the owner announces in the DNS is compared with what the site showed.
func TestAnnounced(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	page := func(host string) string { return get(t, h, "/?host="+host).Body.String() }

	if body := page("announced.example"); !strings.Contains(body, "Announced by the owner: yes.") || strings.Contains(body, "IN TLSA") {
		t.Error("an announced key is not said to be, or its owner is told to announce it")
	} else if strings.Contains(body, "the record is signed.") || !strings.Contains(body, "the record is not signed") {
		t.Error("an unauthenticated answer is said to be signed")
	}
	if body := page("signed.example"); !strings.Contains(body, "Announced by the owner: yes.") || !strings.Contains(body, "And the record is signed.") {
		t.Error("an authenticated answer is not said to be signed")
	}
	stale := stellar.New(sha256.Sum256([]byte("old.example"))).Name
	if body := page("stale.example"); !strings.Contains(body, "Announced by the owner: another key.") || !strings.Contains(body, stale) {
		t.Error("a key the DNS does not announce is not flagged, or the announced sky is not named")
	}
	record := stellar.Record("example.com", sha256.Sum256([]byte("example.com")))
	if body := page("example.com"); !strings.Contains(body, record) || strings.Contains(body, "Announced by the owner") {
		t.Error("a domain that announces nothing is not shown the record to publish")
	}
	if body := page("unread.example"); !strings.Contains(body, "of a form Stellar does not read") {
		t.Error("records of another form are passed over in silence")
	}
	if body := page("deaf.example"); strings.Contains(body, "TLSA") || strings.Contains(body, "Announced") {
		t.Error("a DNS that did not answer is read as an answer")
	}
}

// The public logs are read only when the site allows it and the visitor asks,
// and what they hold is drawn by key.
func TestLogs(t *testing.T) {
	var calls atomic.Int32
	var asked atomic.Int32
	s := New(fakeLookup(&calls))
	h := s.Handler()
	if strings.Contains(get(t, h, "/?host=example.com&logs=1").Body.String(), `id="logs"`) {
		t.Error("a site that does not read the logs offers to")
	}

	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	shown, older, strange := sha256.Sum256([]byte("example.com")), sha256.Sum256([]byte("older")), sha256.Sum256([]byte("strange"))
	s.Logs = func(_ context.Context, host string) ([]Issuance, error) {
		asked.Add(1)
		switch host {
		case "busy.example":
			return nil, errLogsBusy
		case "down.example":
			return nil, errors.New("dial tcp 10.9.9.9:443: refused")
		case "bare.example":
			return nil, nil
		case "strange.example":
			return []Issuance{{Key: strange, NotBefore: now, NotAfter: now.AddDate(0, 3, 0), Issuer: "Other CA"}}, nil
		}
		return []Issuance{
			{Key: older, NotBefore: now.AddDate(0, -2, 0), NotAfter: now.AddDate(0, 1, 0), Issuer: "Example CA"},
			{Key: shown, NotBefore: now.AddDate(0, 0, -3), NotAfter: now.AddDate(0, 3, -3), Issuer: "Example CA"},
			{Key: shown, NotBefore: now.AddDate(0, 0, -3), NotAfter: now.AddDate(0, 3, -3), Issuer: "Example CA"}, // its precertificate
		}, nil
	}

	body := get(t, h, "/?host=example.com").Body.String()
	if !strings.Contains(body, "Look in the public logs") || !strings.Contains(body, `href="/?host=example.com&amp;logs=1#logs"`) || asked.Load() != 0 {
		t.Fatal("the logs are not offered, or were read without being asked for")
	}

	body = get(t, h, "/?host=example.com&logs=1").Body.String()
	first, second := stellar.New(shown).Name, stellar.New(older).Name
	i, j := strings.Index(body, "<strong>"+first+"</strong>"), strings.Index(body, "<strong>"+second+"</strong>")
	if i < 0 || j < 0 || i > j {
		t.Errorf("the two keys are not listed most recent first (%d, %d)", i, j)
	}
	if strings.Count(body, `class="current"`) != 1 || strings.Contains(body, "is not among them") {
		t.Error("the key the site showed is not marked among the logged ones")
	}
	get(t, h, "/?host=example.com&logs=1")
	if asked.Load() != 1 {
		t.Errorf("the logs were asked %d times for one host", asked.Load())
	}

	if body := get(t, h, "/?host=strange.example&logs=1").Body.String(); !strings.Contains(body, "The key this site showed is not among them.") {
		t.Error("a site whose key the logs do not hold is not told so")
	}
	if body := get(t, h, "/?host=bare.example&logs=1").Body.String(); !strings.Contains(body, "hold no valid certificate") {
		t.Error("empty logs are not said to be empty")
	}
	if body := get(t, h, "/?host=busy.example&logs=1").Body.String(); !strings.Contains(body, "accepts no more questions") {
		t.Error("a refusal is not explained")
	}
	if body := get(t, h, "/?host=down.example&logs=1").Body.String(); strings.Contains(body, "10.9.9.9") {
		t.Error("a failure leaks the underlying error")
	}
}

func TestHomeErrors(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()

	rec := get(t, h, "/?host=127.0.0.1")
	if rec.Code != 400 || calls.Load() != 0 {
		t.Errorf("address: status %d, %d lookups", rec.Code, calls.Load())
	}

	rec = get(t, h, "/?host=down.example")
	if rec.Code != 502 {
		t.Errorf("unreachable: status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.1.2.3") {
		t.Error("the error page leaks the underlying network error")
	}

	rec = get(t, h, "/?host="+url.QueryEscape(`"><script>alert(1)</script>`))
	if strings.Contains(rec.Body.String(), "<script>alert") {
		t.Error("the query is echoed unescaped")
	}
}

func TestExplain(t *testing.T) {
	for want, err := range map[int]error{
		400: errNotPublic,
		404: &net.DNSError{IsNotFound: true},
		502: fmt.Errorf("handshake: %w", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}),
		503: errBusy,
		504: context.DeadlineExceeded,
	} {
		key, got := explain(err)
		msg := message("en", key)
		if got != want {
			t.Errorf("explain(%v) = %d, want %d", err, got, want)
		}
		if msg == key {
			t.Errorf("explain(%v) names a sentence nobody wrote: %q", err, key)
		}
		if want == 502 && !strings.Contains(msg, "certificate") {
			t.Errorf("explain(%v) = %q, want a word about the certificate", err, msg)
		}
	}
}

func TestSky(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()

	rec := get(t, h, "/sky/example.com.svg?size=99999")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `width="2048"`) {
		t.Error("size is not clamped")
	}
	if rec := get(t, h, "/sky/example.com"); rec.Code != 404 {
		t.Errorf("no extension: status %d", rec.Code)
	}
	if rec := get(t, h, "/sky/localhost.svg"); rec.Code != 400 {
		t.Errorf("localhost: status %d", rec.Code)
	}
}

// A link to a page is illustrated by a picture of the sky it shows, when the
// site knows the absolute address to give for it.
func TestCard(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	h := s.Handler()

	rec := get(t, h, "/card/example.com.png")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	img, err := png.Decode(rec.Body)
	if err != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
		t.Fatalf("picture: %v", err)
	}
	if rec := get(t, h, "/card/localhost.png"); rec.Code != 400 {
		t.Errorf("localhost: status %d", rec.Code)
	}
	if rec := get(t, h, "/card/example.com"); rec.Code != 404 {
		t.Errorf("no extension: status %d", rec.Code)
	}
	if strings.Contains(get(t, h, "/?host=example.com").Body.String(), "og:image") {
		t.Error("a site that does not know its address names a picture")
	}

	s.BaseURL = "https://stellar.example"
	if body := get(t, h, "/?host=example.com").Body.String(); !strings.Contains(body, `property="og:image" content="https://stellar.example/card/example.com.png"`) {
		t.Error("a host's page does not name its picture")
	}
	if body := get(t, h, "/").Body.String(); !strings.Contains(body, `content="https://stellar.example/card/stellar.example.png"`) {
		t.Error("the empty page does not name the site's own picture")
	}
}

func TestKey(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	post := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/key", strings.NewReader(url.Values{"key": {key}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := post("SHA256:kqckL3tw6x8R1HqM3pohRjFi/6wO3yM2MXYGcV0/2qc")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Subevia Zidensis") {
		t.Errorf("fingerprint: status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `data-host=""`) {
		t.Error("a pasted key must not be remembered as a host")
	}
	if rec := post("not a key"); rec.Code != 422 {
		t.Errorf("garbage: status %d", rec.Code)
	}
	if rec := post(strings.Repeat("A", maxKeyBytes+1)); rec.Code != 422 {
		t.Errorf("oversized: status %d", rec.Code)
	}
}

func TestBusy(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, maxLookups)
	s := New(func(_ context.Context, host string) (stellar.Host, error) {
		started <- struct{}{}
		<-release
		return stellar.Host{}, nil
	})
	h := s.Handler()
	for i := 0; i < maxLookups; i++ {
		go get(t, h, "/?host=h"+string(rune('a'+i))+".example")
	}
	for i := 0; i < maxLookups; i++ {
		<-started
	}
	if rec := get(t, h, "/?host=one-too-many.example"); rec.Code != 503 {
		t.Errorf("status %d, want 503", rec.Code)
	}
	close(release)
}

func TestStatic(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	for _, p := range []string{
		"/static/app.css", "/static/app.js", "/static/favicon.svg", "/healthz",
		"/static/fonts/bricolage-grotesque-300-normal.woff2", "/static/fonts/bricolage-grotesque-400-normal.woff2",
		"/static/fonts/ibm-plex-mono-500-normal.woff2", "/static/fonts/instrument-serif-400-italic.woff2",
	} {
		if rec := get(t, h, p); rec.Code != 200 {
			t.Errorf("%s: status %d", p, rec.Code)
		}
	}
	if rec := get(t, h, "/nope"); rec.Code != 404 {
		t.Errorf("/nope: status %d", rec.Code)
	}
}

// The pages link to the files of the site under their version, where they
// may be kept for ever; any other address serves them for an hour.
func TestStaticVersion(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	body := get(t, h, "/").Body.String()
	base := "/static/" + staticVersion()
	for _, f := range []string{"/app.css", "/app.js", "/favicon.svg"} {
		if !strings.Contains(body, `"`+base+f+`"`) {
			t.Errorf("the page does not link to %s%s", base, f)
		}
	}
	for path, want := range map[string]string{
		base + "/app.css": "public, max-age=31536000, immutable",
		base + "/fonts/ibm-plex-mono-500-normal.woff2": "public, max-age=31536000, immutable",
		"/static/app.css":                              "public, max-age=3600",
		"/static/0123456789ab/app.js":                  "public, max-age=3600",
		"/static/fonts/ibm-plex-mono-500-normal.woff2": "public, max-age=3600",
	} {
		rec := get(t, h, path)
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != want {
			t.Errorf("%s: status %d, Cache-Control %q, want %q", path, rec.Code, rec.Header().Get("Cache-Control"), want)
		}
	}
	if rec := get(t, h, base+"/nope.css"); rec.Code != 404 {
		t.Errorf("a missing file under the version: status %d", rec.Code)
	}
}

// Text travels gzipped to whoever accepts it, and comes back whole; pictures
// and fonts go as they are.
func TestCompress(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	fetch := func(path, accept string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if accept != "" {
			req.Header.Set("Accept-Encoding", accept)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for _, path := range []string{"/?host=example.com", "/static/app.css", "/static/app.js", "/sky/example.com.svg"} {
		plain, zipped := fetch(path, ""), fetch(path, "gzip, br")
		if plain.Header().Get("Content-Encoding") != "" {
			t.Errorf("%s: compressed for a client that did not ask", path)
		}
		if zipped.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(strings.Join(zipped.Header().Values("Vary"), ","), "Accept-Encoding") {
			t.Errorf("%s: not gzipped: %v", path, zipped.Header())
			continue
		}
		r, err := gzip.NewReader(zipped.Body)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		whole, _ := io.ReadAll(r)
		if path != "/?host=example.com" && string(whole) != plain.Body.String() {
			t.Errorf("%s: the gzipped answer is not the plain one", path)
		}
		if zipped.Body.Len() != 0 || len(whole)*2 < plain.Body.Len() {
			t.Errorf("%s: %d bytes came back from %d", path, len(whole), plain.Body.Len())
		}
	}
	for _, path := range []string{"/sky/example.com.png", "/static/fonts/ibm-plex-mono-500-normal.woff2", "/healthz", "/nope"} {
		if rec := fetch(path, "gzip"); rec.Header().Get("Content-Encoding") != "" {
			t.Errorf("%s: compressed, and should not be", path)
		}
	}
}

// The site forbids what it does not use, and asks for https once it knows it
// is served that way.
func TestSecurityHeaders(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	rec := get(t, s.Handler(), "/")
	if rec.Header().Get("Permissions-Policy") == "" || rec.Header().Get("Cross-Origin-Opener-Policy") != "same-origin" {
		t.Errorf("headers: %v", rec.Header())
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("a site that does not know it is served over https asks for it")
	}
	s.BaseURL = "https://stellar.example"
	if got := get(t, s.Handler(), "/").Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("asked under another name, the site promises https for it: %q", got)
	}
	req := httptest.NewRequest("GET", "https://stellar.example/", nil)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("Strict-Transport-Security = %q", got)
	}
}

// The music is off unless the site was started with one. With one, the page
// carries its switch and its credit, and the policy opens to that host alone.
func TestMusic(t *testing.T) {
	var calls atomic.Int32
	quiet := get(t, New(fakeLookup(&calls)).Handler(), "/")
	if strings.Contains(quiet.Body.String(), `id="sound"`) || strings.Contains(quiet.Header().Get("Content-Security-Policy"), "media-src") {
		t.Error("a site without music offers some")
	}

	s := New(fakeLookup(&calls))
	s.Music, s.MusicCredit = "https://archive.example/download/theme.mp3", "A theme, read from archive.example"
	rec := get(t, s.Handler(), "/")
	for _, want := range []string{`id="sound"`, `data-src="https://archive.example/download/theme.mp3"`, "A theme, read from archive.example", "Cut the sound"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "media-src https://archive.example https://*.archive.example") {
		t.Errorf("CSP = %q", csp)
	}
}

// A host's page shows its gate: the six signs of its key, and, once the site
// knows its own key, its own sky as the origin.
func TestGate(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	body := get(t, s.Handler(), "/?host=example.com").Body.String()
	for _, k := range stellar.Gate(sha256.Sum256([]byte("example.com"))) {
		if !strings.Contains(body, `<li title="`+stellar.Sign(k).Name+`">`) {
			t.Errorf("the gate lacks the sign %s", stellar.Sign(k).Name)
		}
	}
	if !strings.Contains(body, "<span>Chevron 1</span>") || !strings.Contains(body, "<span>Chevron 6</span>") || strings.Contains(body, "Chevron 7") {
		t.Error("the six signs are not numbered as chevrons 1 to 6")
	}
	if strings.Contains(body, `class="origin"`) {
		t.Error("a site that does not know its own key shows an origin")
	}

	s.BaseURL = "https://stellar.example"
	s.RefreshSelf()
	body = get(t, s.Handler(), "/?host=example.com").Body.String()
	own := stellar.New(sha256.Sum256([]byte("stellar.example"))).Name
	if !strings.Contains(body, `<li class="origin" title="`+own+`">`) {
		t.Errorf("the origin is not the site's own sky, %s", own)
	}
	if !strings.Contains(body, "<span>Chevron 7</span><span>origin</span>") {
		t.Error("the origin is not chevron 7")
	}
}

// The chart shows the 64 signs and the place of the key, and the place of the
// site outside the sphere once the site knows its own key and looks elsewhere.
func TestChart(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	body := get(t, s.Handler(), "/?host=example.com").Body.String()
	start := strings.Index(body, `<section class="chart"`)
	if start < 0 {
		t.Fatal("no chart")
	}
	alone := body[start:]
	if got := strings.Count(alone, "data-p="); got != stellar.GateCatalogue+1+stellar.GateSigns/2 {
		t.Errorf("%d points, want the %d signs, the arrival and a foot on each line", got, stellar.GateCatalogue)
	}
	if got := strings.Count(alone, `class="tie"`); got != stellar.GateSigns/2 {
		t.Errorf("%d ties from the arrival to its lines, want %d", got, stellar.GateSigns/2)
	}
	// The arrival is the middle of the points nearest to it on its lines.
	digest := sha256.Sum256([]byte("example.com"))
	here, gate := stellar.Place(digest), stellar.Gate(digest)
	var middle [3]float64
	for i := 0; i < stellar.GateSigns; i += 2 {
		foot := nearest(here, stellar.SignPlace(gate[i]), stellar.SignPlace(gate[i+1]))
		for j := range middle {
			middle[j] += foot[j] / 3
		}
	}
	for j := range middle {
		if math.Abs(middle[j]-here[j]) > 1e-4 {
			t.Errorf("the arrival %v is not the middle of its three feet %v", here, middle)
		}
	}
	if got := strings.Count(alone, `class="lit"`); got != stellar.GateSigns {
		t.Errorf("%d signs lit, want %d", got, stellar.GateSigns)
	}
	if got := strings.Count(alone, `class="chord"`); got != stellar.GateSigns/2 {
		t.Errorf("%d lines, want %d", got, stellar.GateSigns/2)
	}
	if strings.Contains(alone, `class="from"`) || strings.Contains(alone, `class="trip"`) {
		t.Error("a site that does not know its own key shows an origin")
	}

	s.BaseURL = "https://stellar.example"
	s.RefreshSelf()
	body = get(t, s.Handler(), "/?host=example.com").Body.String()
	if !strings.Contains(body, `class="from"`) || !strings.Contains(body, `class="trip"`) {
		t.Error("the chart lacks the origin and the trip")
	}
	origin := stellar.Place(sha256.Sum256([]byte("stellar.example")))
	if p := away(origin); math.Abs(math.Sqrt(p[0]*p[0]+p[1]*p[1]+p[2]*p[2])-chartAway) > 1e-9 {
		t.Errorf("the origin stands at %v, not outside the sphere", p)
	}
	// Taken away, the chart carries its own colours and no class.
	file := get(t, s.Handler(), "/chart/example.com.svg")
	taken := file.Body.String()
	if file.Code != 200 || file.Header().Get("Content-Type") != "image/svg+xml" || strings.Contains(taken, "class=") ||
		!strings.Contains(taken, `stroke-dasharray="3 3"`) || !strings.Contains(taken, ">ORIGIN<") ||
		!strings.Contains(taken, strings.ToUpper(stellar.New(sha256.Sum256([]byte("example.com"))).Name)) {
		t.Errorf("the chart as a file: status %d, %.300s", file.Code, taken)
	}
	if !strings.Contains(body, `href="/chart/example.com.svg"`) {
		t.Error("the page does not offer the chart as a file")
	}
	// Looking at itself, the site is both ends: there is no trip to draw.
	body = get(t, s.Handler(), "/?host=stellar.example").Body.String()
	if strings.Contains(body, `class="from"`) || strings.Contains(body, `class="trip"`) {
		t.Error("the site draws a trip to itself")
	}
}

// A stardate is the year and the share of it already gone, never rounded up
// into the next year.
func TestStardate(t *testing.T) {
	for moment, want := range map[string]string{
		"2026-01-01T00:00:00Z":      "2026.00",
		"2026-07-02T12:00:00Z":      "2026.50",
		"2026-11-26T10:00:00Z":      "2026.90",
		"2026-12-31T23:59:59Z":      "2026.99",
		"2028-12-31T23:59:59+02:00": "2028.99",
	} {
		at, err := time.Parse(time.RFC3339, moment)
		if err != nil {
			t.Fatal(err)
		}
		if got := stardate(at); got != want {
			t.Errorf("stardate(%s) = %s, want %s", moment, got, want)
		}
	}
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	s.now = func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC) }
	body := get(t, s.Handler(), "/?host=example.com&lang=fr").Body.String()
	if !strings.Contains(body, `<dt>Date stellaire</dt><dd class="mono stardate" title="2026-11-26 10:00 UTC"><svg`) {
		t.Error("the page does not give its stardate")
	}
	if !strings.Contains(body, `</svg>2026.90</dd>`) {
		t.Error("the stardate is not 2026.90")
	}
}

// Another host can be placed on the chart: its point, the line to it, and
// how far apart the two keys stand.
func TestCompare(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	body := get(t, h, "/?host=example.com").Body.String()
	if strings.Contains(body, `class="other"`) || !strings.Contains(body, `name="with"`) {
		t.Error("a page with nothing to compare draws another key, or offers no way to")
	}
	body = get(t, h, "/?host=example.com&with=Example.ORG").Body.String()
	a, b := sha256.Sum256([]byte("example.com")), sha256.Sum256([]byte("example.org"))
	want := strconv.FormatFloat(stellar.Apart(a, b), 'f', 2, 64)
	for _, part := range []string{`class="other"`, `class="span"`, ">example.org</text>", stellar.New(b).Name, want + " radius", `with=example.org`} {
		if !strings.Contains(body, part) {
			t.Errorf("the comparison lacks %q", part)
		}
	}
	if d := stellar.Apart(a, b); d <= 0 || d > 2 {
		t.Errorf("the two places are %v apart", d)
	}
	// The same host again is the same key: one place, and the page says so.
	body = get(t, h, "/?host=example.com&with=example.com").Body.String()
	if strings.Contains(body, `class="other"`) || !strings.Contains(body, "serves the same key") {
		t.Error("the same key is drawn twice, or not said to be the same")
	}
	// A host that cannot be read leaves the chart alone and says why.
	rec := get(t, h, "/?host=example.com&with=down.example")
	body = rec.Body.String()
	if rec.Code != 200 || strings.Contains(body, `class="other"`) || !strings.Contains(body, `class="note with bad"`) {
		t.Errorf("an unreachable host to compare: status %d", rec.Code)
	}
	if file := get(t, h, "/chart/example.com.svg?with=example.org").Body.String(); !strings.Contains(file, ">EXAMPLE.ORG</text>") || strings.Contains(file, "class=") {
		t.Error("the chart as a file does not carry the other key")
	}
}

// A key the visitor was given is compared, whole, with the key the host
// serves: confirmed when they are the same, refused when they are not, and a
// private key is turned away unread.
func TestVerify(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	post := func(host, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/verify", strings.NewReader(url.Values{"host": {host}, "key": {key}, "lang": {"en"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	served := stellar.Key{Digest: sha256.Sum256([]byte("example.com"))}
	other := stellar.Key{Digest: sha256.Sum256([]byte("example.org"))}

	if body := get(t, h, "/?host=example.com").Body.String(); !strings.Contains(body, `action="/verify#verify"`) || strings.Contains(body, `class="verdict"`) {
		t.Error("the page of a host offers no check, or shows a verdict nobody asked for")
	}
	if strings.Contains(get(t, h, "/").Body.String(), `id="verify"`) {
		t.Error("the empty page offers a key to check, against nothing")
	}

	rec := post("example.com", served.Fingerprint())
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `data-match="yes"`) || !strings.Contains(body, "Key confirmed.") {
		t.Errorf("the key the host serves is not confirmed: status %d", rec.Code)
	}
	if got := strings.Count(body, `<li class="ok"`); got != stellar.GateSigns || strings.Contains(body, `<li class="bad"`) {
		t.Errorf("%d signs lit green for the same key, want %d", got, stellar.GateSigns)
	}
	var numbers []string
	for _, k := range stellar.Gate(served.Digest) {
		numbers = append(numbers, strconv.Itoa(k))
	}
	if !strings.Contains(body, `data-gate="`+strings.Join(numbers, ",")+`"`) || !strings.Contains(body, `data-signs="/signs/`+signs().name+`"`) {
		t.Error("the page does not give the script the signs to stop on, or their picture")
	}

	rec = post("example.com", other.Fingerprint())
	body = rec.Body.String()
	differ := 0
	for i, k := range stellar.Gate(other.Digest) {
		if k != stellar.Gate(served.Digest)[i] {
			differ++
		}
	}
	if rec.Code != 200 || !strings.Contains(body, `data-match="no"`) || !strings.Contains(body, "This is not the key example.com serves.") ||
		!strings.Contains(body, fmt.Sprintf("and %d of the six signs", differ)) ||
		strings.Count(body, `<li class="bad"`) != differ || !strings.Contains(body, stellar.New(other.Digest).Name) || !strings.Contains(body, other.Fingerprint()) {
		t.Errorf("another key is not refused as it should: status %d, %d signs differ", rec.Code, differ)
	}

	rec = post("example.com", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\n-----END OPENSSH PRIVATE KEY-----")
	if body = rec.Body.String(); rec.Code != 422 || !strings.Contains(body, "This is a private key.") || strings.Contains(body, `data-match="yes"`) || strings.Contains(body, "b3BlbnNzaC1rZXk") {
		t.Errorf("a private key: status %d", rec.Code)
	}
	if rec = post("example.com", "not a key"); rec.Code != 422 || !strings.Contains(rec.Body.String(), `data-match="error"`) {
		t.Errorf("something that is no key: status %d", rec.Code)
	}
	if rec = post("down.example", served.Fingerprint()); rec.Code == 200 || strings.Contains(rec.Body.String(), `class="verdict"`) {
		t.Errorf("a host that cannot be read gives a verdict: status %d", rec.Code)
	}

	// The same check asked by a link, as the browser extension asks it.
	rec = get(t, h, "/verify?host=example.com&key="+url.QueryEscape(served.Fingerprint()))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `data-match="yes"`) {
		t.Errorf("a key given in the address is not checked: status %d", rec.Code)
	}

	// The picture of the catalogue, which the script spins through.
	sheet := signs()
	rec = get(t, h, "/signs/"+sheet.name)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("the picture of the catalogue: status %d, %v", rec.Code, rec.Header())
	}
	if got := strings.Count(rec.Body.String(), `viewBox="60 20 880 880"`); got != stellar.GateCatalogue || strings.Contains(rec.Body.String(), "currentColor") {
		t.Errorf("the picture holds %d signs, want %d, in the ink of the page", got, stellar.GateCatalogue)
	}
	if rec := get(t, h, "/signs/signs-000000000000.svg"); rec.Code != 404 {
		t.Errorf("another name for the picture: status %d", rec.Code)
	}
}

// Many visitors asking for one picture at once get it drawn once: those who
// waited for their turn take what the first one drew.
func TestPictureIsDrawnOnce(t *testing.T) {
	var calls atomic.Int32
	s := New(fakeLookup(&calls))
	h := s.Handler()
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := get(t, h, "/card/example.com.png"); rec.Code != 200 {
				t.Errorf("status %d", rec.Code)
			}
		}()
	}
	wg.Wait()
	all := time.Since(start)
	start = time.Now()
	stellar.New(sha256.Sum256([]byte("example.com"))).PNG(cardWidth, cardHeight)
	one := time.Since(start)
	// Drawn once, the 24 answers cost about one drawing; drawn each in turn
	// they would cost 24.
	if all > 6*one+200*time.Millisecond {
		t.Errorf("24 requests for one picture took %v, one drawing takes %v", all, one)
	}
}

// The sky can be taken away, as an SVG or as a square PNG.
func TestDownload(t *testing.T) {
	var calls atomic.Int32
	h := New(fakeLookup(&calls)).Handler()
	body := get(t, h, "/?host=example.com").Body.String()
	for _, want := range []string{`href="/sky/example.com.svg?size=1200" download="example.com.svg"`, `href="/sky/example.com.png" download="example.com.png"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %s", want)
		}
	}
	rec := get(t, h, "/sky/example.com.png")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1024 || b.Dy() != 1024 {
		t.Errorf("size %v", b)
	}
	if rec := get(t, h, "/sky/localhost.png"); rec.Code != 400 {
		t.Errorf("a private name: status %d", rec.Code)
	}
}
