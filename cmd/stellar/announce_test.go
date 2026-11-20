package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mesa-black/stellar"
)

// fakeGandi answers like the LiveDNS API for one zone, and keeps what it was
// asked.
type fakeGandi struct {
	t      *testing.T
	sets   map[string][]string // record name → values
	puts   []string
	status int // forced answer, when not zero
}

func (f *fakeGandi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer secret-token" {
		f.t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
	}
	if f.status != 0 {
		http.Error(w, `{"message":"no"}`, f.status)
		return
	}
	const prefix = "/domains/example.org/records/"
	name, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/TLSA")
	if !strings.HasPrefix(r.URL.Path, prefix) || !ok {
		f.t.Errorf("unexpected path %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case "GET":
		values, ok := f.sets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"rrset_name": name, "rrset_type": "TLSA", "rrset_values": values})
	case "PUT":
		var body struct {
			Values []string `json:"rrset_values"`
			TTL    int      `json:"rrset_ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TTL != 300 {
			f.t.Errorf("bad PUT body: %v, ttl %d", err, body.TTL)
		}
		f.sets[name] = body.Values
		f.puts = append(f.puts, name)
		w.WriteHeader(http.StatusCreated)
	default:
		f.t.Errorf("unexpected method %s", r.Method)
	}
}

func digest(b byte) [32]byte {
	var d [32]byte
	for i := range d {
		d[i] = b
	}
	return d
}

func newAnnouncer(t *testing.T, api *fakeGandi, keys map[string][32]byte) (announcer, *bytes.Buffer) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	out := &bytes.Buffer{}
	return announcer{
		zone: "example.org",
		ttl:  300,
		out:  out,
		inspect: func(_ context.Context, host string) (stellar.Host, error) {
			key, ok := keys[host]
			if !ok {
				return stellar.Host{}, errors.New("connection refused")
			}
			return stellar.Host{Key: stellar.Key{Digest: key}}, nil
		},
		records: &gandi{base: srv.URL, zone: "example.org", token: "secret-token", client: srv.Client()},
	}, out
}

func TestAnnounce(t *testing.T) {
	old, current := digest(0x11), digest(0x22)
	api := &fakeGandi{t: t, sets: map[string][]string{
		// Already right, written the way an API may give it back.
		"_443._tcp": {fmt.Sprintf("3 1 1  %X", current)},
		// The key of the previous certificate.
		"_443._tcp.www": {fmt.Sprintf("3 1 1 %x", old)},
	}}
	a, out := newAnnouncer(t, api, map[string][32]byte{
		"example.org": current, "www.example.org": current, "new.example.org": current,
	})

	if err := a.run(context.Background(), []string{"example.org", "WWW.example.org.", "new.example.org"}); err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if got := strings.Join(api.puts, " "); got != "_443._tcp.www _443._tcp.new" {
		t.Errorf("records written: %q", got)
	}
	want := fmt.Sprintf("3 1 1 %x", current)
	for _, name := range []string{"_443._tcp.www", "_443._tcp.new"} {
		if got := api.sets[name]; len(got) != 1 || got[0] != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, line := range []string{"example.org: ", ", unchanged", "replacing 1 record(s)", "no record before"} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("output lacks %q:\n%s", line, out)
		}
	}
}

// A host that cannot be read keeps its record, does not stop the others, and
// makes the command fail.
func TestAnnounceLeavesUnreachableHostsAlone(t *testing.T) {
	old, current := digest(0x11), digest(0x22)
	api := &fakeGandi{t: t, sets: map[string][]string{
		"_443._tcp.down": {fmt.Sprintf("3 1 1 %x", old)},
	}}
	a, out := newAnnouncer(t, api, map[string][32]byte{"up.example.org": current})

	err := a.run(context.Background(), []string{"down.example.org", "elsewhere.test", "up.example.org"})
	if err == nil || !strings.Contains(err.Error(), "2 of 3") {
		t.Fatalf("err = %v", err)
	}
	if got := api.sets["_443._tcp.down"][0]; got != fmt.Sprintf("3 1 1 %x", old) {
		t.Errorf("record of the unreachable host changed: %s", got)
	}
	if len(api.sets["_443._tcp.up"]) != 1 {
		t.Errorf("the reachable host was not announced:\n%s", out)
	}
	if !strings.Contains(out.String(), "elsewhere.test: FAILED: not in the zone example.org") {
		t.Errorf("output:\n%s", out)
	}
}

func TestAnnounceReportsRefusals(t *testing.T) {
	api := &fakeGandi{t: t, status: http.StatusForbidden}
	a, out := newAnnouncer(t, api, map[string][32]byte{"example.org": digest(0x22)})
	if err := a.run(context.Background(), []string{"example.org"}); err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(out.String(), "Gandi answered 403") || strings.Contains(out.String(), "secret-token") {
		t.Errorf("output:\n%s", out)
	}
}

func TestAnnounceDryRun(t *testing.T) {
	out := &bytes.Buffer{}
	a := announcer{zone: "example.org", ttl: 300, out: out, inspect: func(context.Context, string) (stellar.Host, error) {
		return stellar.Host{Key: stellar.Key{Digest: digest(0x22)}}, nil
	}}
	if err := a.run(context.Background(), []string{"www.example.org"}); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("_443._tcp.www.example.org. IN TLSA 3 1 1 %x", digest(0x22)); !strings.Contains(out.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}
}

func TestRecordName(t *testing.T) {
	for host, want := range map[string]string{
		"example.org":       "_443._tcp",
		"www.example.org":   "_443._tcp.www",
		"a.b.example.org":   "_443._tcp.a.b",
		"notexample.org":    "",
		"example.org.evil.": "",
	} {
		got, err := recordName(host, "example.org")
		if got != want || (err == nil) != (want != "") {
			t.Errorf("recordName(%q) = %q, %v; want %q", host, got, err, want)
		}
	}
}
