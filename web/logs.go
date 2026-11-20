package web

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/mesa-black/stellar"
)

// Every certificate a public authority issues is written in public logs
// (Certificate Transparency). Read by key, those logs answer a question the
// site itself cannot: which keys hold, today, a valid certificate for this
// name? More than one is ordinary around a renewal. One nobody recognises is
// a certificate somebody obtained for the name.
//
// The logs are read through Cert Spotter (SSLMate), whose answer already
// carries the SHA-256 of each certificate's key: the digest a sky is drawn
// from. Two things follow from using somebody else's service, and the page
// says both. The name the visitor typed is sent to it, so nothing is asked
// until the visitor asks. And it answers with the certificates that have not
// expired, so this is the present, not a history.

// Issuance is one certificate found in the logs, reduced to what a sky needs.
type Issuance struct {
	Key       [32]byte
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    string
}

// Logs returns the unexpired certificates the public logs hold for a host.
type Logs func(ctx context.Context, host string) ([]Issuance, error)

const (
	logsTTL = 4 * time.Hour
	logsMax = 12
)

var errLogsBusy = errors.New("the log service refuses more questions for now")

// CertSpotter asks SSLMate's Cert Spotter. The address is fixed; only the
// host name, already validated, goes into the query.
func CertSpotter(ctx context.Context, host string) ([]Issuance, error) {
	query := url.Values{"domain": {host}, "match_wildcards": {"true"}, "expand": {"issuer"}}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.certspotter.com/v1/issuances?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "stellar (+https://github.com/mesa-black/stellar)")
	resp, err := logsClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, errLogsBusy
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("log service answered %d", resp.StatusCode)
	}
	var raw []struct {
		Pubkey    string    `json:"pubkey_sha256"`
		NotBefore time.Time `json:"not_before"`
		NotAfter  time.Time `json:"not_after"`
		Issuer    struct {
			Friendly string `json:"friendly_name"`
			Name     string `json:"name"`
		} `json:"issuer"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]Issuance, 0, len(raw))
	for _, r := range raw {
		key, err := hex.DecodeString(r.Pubkey)
		if err != nil || len(key) != 32 {
			continue
		}
		issuer := r.Issuer.Friendly
		if issuer == "" {
			issuer = r.Issuer.Name
		}
		out = append(out, Issuance{Key: [32]byte(key), NotBefore: r.NotBefore, NotAfter: r.NotAfter, Issuer: issuer})
	}
	return out, nil
}

// logsClient reaches public addresses only, like every connection this
// server opens, and follows no redirection.
var logsClient = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 4 * time.Second, Control: publicOnly}).DialContext,
		TLSHandshakeTimeout: 4 * time.Second,
		MaxIdleConns:        2,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type cachedLogs struct {
	issuances []Issuance
	at        time.Time
}

// loggedSky is one key found in the logs, as the page shows it.
type loggedSky struct {
	Name        string
	Fingerprint string
	SVG         template.HTML
	Since       string
	Until       string
	Issuer      string
	Count       int
	Current     bool
}

// logged asks the logs for a host, through a cache: the service allows a
// hundred questions an hour, and its answers change a few times a quarter.
func (s *Server) logged(ctx context.Context, host string) ([]Issuance, error) {
	s.mu.Lock()
	hit, ok := s.logs[host]
	s.mu.Unlock()
	if ok && s.now().Sub(hit.at) < logsTTL {
		return hit.issuances, nil
	}
	ctx, cancel := context.WithTimeout(ctx, lookupBudget)
	defer cancel()
	issuances, err := s.Logs(ctx, host)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if len(s.logs) >= cacheMax {
		clear(s.logs)
	}
	s.logs[host] = cachedLogs{issuances, s.now()}
	s.mu.Unlock()
	return issuances, nil
}

// showLogs puts on the page the keys that hold a valid certificate for the
// host, most recent first, and says whether the key the site showed is one
// of them.
func (v *view) showLogs(issuances []Issuance, seen [32]byte) {
	type span struct {
		since, until time.Time
		issuer       string
		count        int
	}
	byKey := map[[32]byte]*span{}
	var order [][32]byte
	for _, is := range issuances {
		sp, ok := byKey[is.Key]
		if !ok {
			sp = &span{since: is.NotBefore, until: is.NotAfter, issuer: is.Issuer}
			byKey[is.Key] = sp
			order = append(order, is.Key)
		}
		if is.NotBefore.Before(sp.since) {
			sp.since = is.NotBefore
		}
		if is.NotAfter.After(sp.until) {
			sp.until = is.NotAfter
		}
		sp.count++
	}
	sort.SliceStable(order, func(a, b int) bool { return byKey[order[a]].since.After(byKey[order[b]].since) })
	if len(order) > logsMax {
		order = order[:logsMax]
	}

	v.LogsState = "empty"
	for _, key := range order {
		sp := byKey[key]
		c := stellar.New(key)
		sky := loggedSky{
			Name:        c.Name,
			Fingerprint: stellar.Key{Digest: key}.Fingerprint(),
			SVG:         template.HTML(c.SVG(200, "")),
			Since:       longDate(v.Lang, sp.since),
			Until:       longDate(v.Lang, sp.until),
			Issuer:      sp.issuer,
			Count:       sp.count,
			Current:     key == seen,
		}
		if sky.Current {
			v.LogsCurrent = true
		}
		v.Logs = append(v.Logs, sky)
		v.LogsState = "shown"
	}
}
