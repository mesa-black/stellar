package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mesa-black/stellar"
)

// announce keeps the DANE records of a few hosts equal to the keys those
// hosts serve. It is meant to run every few minutes, next to servers whose
// certificates renew with a new key each time: the record follows the key,
// and nobody has to remember it.
//
// It reads the key the way a visitor does, from a TLS handshake whose
// certificate chain verifies. It never sees a private key, and a host that
// does not answer, or answers with a certificate this machine does not
// trust, is left as it was: nothing is ever deleted.
//
// Two things to know before relying on it. Between a renewal and the next
// run, plus the record's lifetime in caches, the DNS announces the previous
// key: a few minutes, each time a certificate renews. And the record set at
// _443._tcp.<host> belongs to this command: whatever else was published
// there is replaced.
//
// The zone is edited through Gandi's LiveDNS API, with a token read from the
// environment (GANDI_TOKEN) so that it appears in no command line.
func announce(args []string) error {
	fs := flag.NewFlagSet("stellar announce", flag.ExitOnError)
	zone := fs.String("zone", "", "the DNS `zone` at Gandi the hosts belong to, e.g. example.org")
	ttl := fs.Int("ttl", 300, "lifetime of the records, in `seconds` (Gandi accepts 300 and more)")
	dry := fs.Bool("dry-run", false, "print the records and publish nothing; needs no token")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: GANDI_TOKEN=… stellar announce -zone <zone> [flags] <host>...")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if *zone == "" || fs.NArg() == 0 {
		fs.Usage()
		os.Exit(2)
	}
	if *ttl < 300 {
		return fmt.Errorf("-ttl %d: Gandi accepts 300 seconds and more", *ttl)
	}

	a := announcer{
		zone: strings.ToLower(strings.TrimSuffix(*zone, ".")),
		ttl:  *ttl,
		out:  os.Stdout,
		inspect: func(ctx context.Context, host string) (stellar.Host, error) {
			return stellar.InspectHost(ctx, nil, host, false)
		},
	}
	if !*dry {
		token := os.Getenv("GANDI_TOKEN")
		if token == "" {
			return errors.New("GANDI_TOKEN is not set (or use -dry-run)")
		}
		a.records = &gandi{base: "https://api.gandi.net/v5/livedns", zone: a.zone, token: token, client: gandiClient}
	}
	return a.run(context.Background(), fs.Args())
}

// records is the part of a DNS provider announce needs: the TLSA record set
// at one name, read and replaced.
type records interface {
	get(ctx context.Context, name string) ([]string, error)
	put(ctx context.Context, name string, values []string, ttl int) error
}

type announcer struct {
	zone    string
	ttl     int
	out     io.Writer
	inspect func(ctx context.Context, host string) (stellar.Host, error)
	// records is nil for a dry run.
	records records
}

// run handles every host, and reports the failures at the end: one host that
// is down does not keep the others from being announced.
func (a announcer) run(ctx context.Context, hosts []string) error {
	failed := 0
	for _, host := range hosts {
		if err := a.host(ctx, host); err != nil {
			fmt.Fprintf(a.out, "%s: FAILED: %v\n", host, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d hosts not announced", failed, len(hosts))
	}
	return nil
}

func (a announcer) host(ctx context.Context, host string) error {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	name, err := recordName(host, a.zone)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	seen, err := a.inspect(ctx, host)
	if err != nil {
		return err
	}
	sky := stellar.New(seen.Key.Digest).Name
	want := fmt.Sprintf("3 1 1 %x", seen.Key.Digest)

	if a.records == nil {
		fmt.Fprintf(a.out, "%s: %s\n  %s\n", host, sky, stellar.Record(host, seen.Key.Digest))
		return nil
	}
	have, err := a.records.get(ctx, name)
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	if len(have) == 1 && sameRecord(have[0], want) {
		fmt.Fprintf(a.out, "%s: %s, unchanged\n", host, sky)
		return nil
	}
	if err := a.records.put(ctx, name, []string{want}, a.ttl); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	fmt.Fprintf(a.out, "%s: %s, published (%s)\n", host, sky, was(have))
	return nil
}

// recordName returns the name of a host's TLSA record set, relative to the
// zone: _443._tcp for the zone itself, _443._tcp.www for www.<zone>.
func recordName(host, zone string) (string, error) {
	switch {
	case host == zone:
		return "_443._tcp", nil
	case strings.HasSuffix(host, "."+zone):
		return "_443._tcp." + strings.TrimSuffix(host, "."+zone), nil
	}
	return "", fmt.Errorf("not in the zone %s", zone)
}

// sameRecord compares two TLSA values the way the DNS does: fields apart
// from spacing, hexadecimal apart from case.
func sameRecord(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

// was describes what a record set held before it was replaced.
func was(have []string) string {
	if len(have) == 0 {
		return "no record before"
	}
	return fmt.Sprintf("replacing %d record(s)", len(have))
}

// gandi edits one zone through Gandi's LiveDNS API.
type gandi struct {
	base   string
	zone   string
	token  string
	client *http.Client
}

// gandiClient follows no redirection: the token goes to the address written
// above and nowhere else.
var gandiClient = &http.Client{
	Timeout:       15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func (g *gandi) url(name string) string {
	return g.base + "/domains/" + url.PathEscape(g.zone) + "/records/" + url.PathEscape(name) + "/TLSA"
}

func (g *gandi) do(ctx context.Context, method, name string, body any) (*http.Response, error) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.url(name), payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return g.client.Do(req)
}

// get returns the TLSA values at a name; none when there is no record.
func (g *gandi) get(ctx context.Context, name string) ([]string, error) {
	resp, err := g.do(ctx, "GET", name, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, nil
	case resp.StatusCode != http.StatusOK:
		return nil, gandiError(resp)
	}
	var rrset struct {
		Values []string `json:"rrset_values"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rrset); err != nil {
		return nil, err
	}
	return rrset.Values, nil
}

// put replaces the TLSA record set at a name.
func (g *gandi) put(ctx context.Context, name string, values []string, ttl int) error {
	resp, err := g.do(ctx, "PUT", name, map[string]any{"rrset_values": values, "rrset_ttl": ttl})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return gandiError(resp)
	}
	return nil
}

// gandiError keeps the status and the start of what the API said. The token
// is in the request, never in an answer.
func gandiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	return fmt.Errorf("Gandi answered %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
