package web

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mesa-black/stellar"
)

func fakeLookup(calls *atomic.Int32) Lookup {
	return func(_ context.Context, host string) (stellar.Key, error) {
		calls.Add(1)
		if host == "down.example" {
			return stellar.Key{}, errors.New("dial tcp 10.1.2.3:443: connection refused")
		}
		return stellar.Key{Digest: sha256.Sum256([]byte(host)), Kind: "X.509 certificate"}, nil
	}
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
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Every key has a sky") {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<svg") {
		t.Error("the empty page draws a constellation")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "font-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
	for _, link := range []string{`href="https://github.com/mesa-black/sablier"`, `href="https://mesa.black"`} {
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
		msg, got := explain(err)
		if got != want {
			t.Errorf("explain(%v) = %d, want %d", err, got, want)
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
	s := New(func(_ context.Context, host string) (stellar.Key, error) {
		started <- struct{}{}
		<-release
		return stellar.Key{}, nil
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
		"/static/app.css", "/static/app.js", "/healthz",
		"/static/fonts/bricolage-grotesque-400-normal.woff2", "/static/fonts/ibm-plex-mono-500-normal.woff2",
	} {
		if rec := get(t, h, p); rec.Code != 200 {
			t.Errorf("%s: status %d", p, rec.Code)
		}
	}
	if rec := get(t, h, "/nope"); rec.Code != 404 {
		t.Errorf("/nope: status %d", rec.Code)
	}
}
