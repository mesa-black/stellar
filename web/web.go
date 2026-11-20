// Package web serves Stellar as a small website: type a host name, see the
// constellation of the key its certificate carries.
package web

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mesa-black/stellar"
)

//go:embed templates/page.html
var pageHTML string

//go:embed static
var staticFS embed.FS

var page = template.Must(template.New("page").Parse(pageHTML))

const (
	cacheTTL     = 5 * time.Minute
	cacheMax     = 2048
	maxLookups   = 16
	lookupBudget = 8 * time.Second
	maxKeyBytes  = 64 << 10
	// The size link previews expect, and how many pictures are kept.
	cardWidth, cardHeight = 1200, 630
	cardsMax              = 128
)

// Lookup returns what a host shows of itself: its key and how it negotiates.
type Lookup func(ctx context.Context, host string) (stellar.Host, error)

// Server is the website. The zero value is not usable; call New.
type Server struct {
	// BaseURL is the public address of the site, without a trailing slash,
	// e.g. "https://stellar.example". When set, shared messages end with a
	// link back to the sky; when empty, they carry no link, since an address
	// guessed from the request could be a local one.
	BaseURL string

	// Logs reads the public certificate logs. Nil turns the feature off:
	// asking them sends a visitor's host name to a third party, which is a
	// decision for whoever runs the site.
	Logs Logs

	lookup Lookup
	now    func() time.Time
	slots  chan struct{}
	// selfBusy is set while the site's own key is being fetched again.
	selfBusy atomic.Bool

	mu    sync.Mutex
	cache map[string]cached
	// cards holds the preview pictures already drawn, by key.
	cards map[[32]byte][]byte
	// logs holds what the public logs said about each host.
	logs map[string]cachedLogs
}

type cached struct {
	host stellar.Host
	at   time.Time
}

// New returns a server that resolves hosts with lookup. Pass PublicLookup
// unless you know every caller is trusted.
func New(lookup Lookup) *Server {
	return &Server{
		lookup: lookup,
		now:    time.Now,
		slots:  make(chan struct{}, maxLookups),
		cache:  map[string]cached{},
		cards:  map[[32]byte][]byte{},
		logs:   map[string]cachedLogs{},
	}
}

// Handler returns the routes of the site.
func (s *Server) Handler() http.Handler {
	static, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("POST /key", s.key)
	mux.HandleFunc("GET /sky/{file}", s.sky)
	mux.HandleFunc("GET /card/{file}", s.card)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	return headers(mux)
}

// headers sets the security headers. The pages load nothing from elsewhere
// and carry no inline script or style, so the policy can be strict.
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'self'; font-src 'self'; script-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// view is what the page template renders.
type view struct {
	// Lang is the language of the page; LangParam is the same when the
	// visitor chose it, so that links and forms carry the choice.
	Lang      string
	LangParam string
	Langs     []langLink
	// Base is the site's public address, empty when unknown.
	Base string
	// CardHost is the host whose picture illustrates a link to this page.
	CardHost string

	Query string // what the visitor typed, echoed in the form
	Error string

	// Set when there is a constellation to show.
	Host        string // empty for a pasted key
	Name        string
	Fingerprint string
	Kind        string
	Label       string
	SVG         template.HTML

	// Set for a host: what its certificate and its handshake say.
	KeyAlgorithm string
	Issuer       string
	ValidUntil   string
	Version      string
	KeyExchange  string
	PostQuantum  bool

	// Announced says how the key compares with what the owner publishes in
	// the DNS: "yes", "other" (other keys are announced, not this one),
	// "unread" (records of a form Stellar does not read), "none", or empty
	// when the DNS gave no usable answer.
	Announced string
	// AnnouncedSigned says the resolver authenticated the DNS answer (DNSSEC).
	AnnouncedSigned bool
	// AnnouncedNames are the skies announced, when this one is not.
	AnnouncedNames string
	// Record is the DNS record that would announce this key.
	Record string

	// LogsState says what became of a look at the public logs: "offer" (the
	// visitor may ask), "shown", "empty", "busy" or "failed". Empty when the
	// site does not read them.
	LogsState   string
	Logs        []loggedSky
	LogsCurrent bool
	LogsURL     string

	// ShareURL opens Threema with a message about this sky, ready to edit.
	ShareURL string

	// Set on the empty page, when the site knows its own key: the sky of the
	// very site being read.
	SelfHost string
	SelfName string
	SelfSVG  template.HTML
}

// newView starts a page in the language of the request.
func (s *Server) newView(r *http.Request, query string) view {
	v := view{Query: query, Base: s.BaseURL}
	var explicit bool
	if v.Lang, explicit = pickLanguage(r); explicit {
		v.LangParam = v.Lang
	}
	return v
}

// finish sets what depends on the page's final content: the switcher's
// links, which lead to this same page in another language.
func (v *view) finish() {
	for _, code := range languages {
		q := url.Values{}
		if v.Host != "" {
			q.Set("host", v.Host)
		}
		q.Set("lang", code)
		v.Langs = append(v.Langs, langLink{Code: code, Name: languageNames[code], URL: "/?" + q.Encode(), Current: code == v.Lang})
	}
}

// Home is the address of the empty page, in the language chosen.
func (v view) Home() string {
	if v.LangParam == "" {
		return "/"
	}
	return "/?lang=" + v.LangParam
}

// HostURL is the address of a host's page, in the language chosen.
func (v view) HostURL(host string) string {
	q := url.Values{"host": {host}}
	if v.LangParam != "" {
		q.Set("lang", v.LangParam)
	}
	return "/?" + q.Encode()
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("host")
	v := s.newView(r, query)
	status := http.StatusOK
	if fp := strings.TrimSpace(query); strings.HasPrefix(fp, "SHA256:") {
		// A fingerprint typed where a host name goes, as a report that prints
		// one invites: it names a key directly, so there is nothing to fetch
		// and no host to remember.
		key, err := stellar.ParseFingerprint(fp)
		if err != nil {
			v.Error, status = v.S("error.print"), http.StatusUnprocessableEntity
		} else {
			v.Query = fp
			v.show(key)
			v.share("")
		}
	} else if query != "" {
		host, seen, err := s.resolve(r.Context(), query)
		if err != nil {
			var key string
			key, status = explain(err)
			v.Error = v.S(key)
		} else {
			v.Query, v.Host = host, host
			v.show(seen.Key)
			v.KeyAlgorithm, v.Issuer = seen.KeyAlgorithm, seen.Issuer
			v.Version, v.KeyExchange, v.PostQuantum = seen.Version, seen.KeyExchange, seen.PostQuantum
			if !seen.NotAfter.IsZero() {
				v.ValidUntil = longDate(v.Lang, seen.NotAfter)
			}
			v.announce(host, seen)
			if s.Logs != nil {
				// Only when asked: the name goes to a third party.
				v.LogsState, v.LogsURL = "offer", v.HostURL(host)+"&logs=1#logs"
				if r.URL.Query().Get("logs") == "1" {
					issuances, err := s.logged(r.Context(), host)
					switch {
					case errors.Is(err, errLogsBusy):
						v.LogsState = "busy"
					case err != nil:
						v.LogsState = "failed"
					default:
						v.showLogs(issuances, seen.Key.Digest)
					}
				}
			}
			v.share(s.BaseURL)
		}
	}
	if v.Name == "" && v.Error == "" {
		s.showSelf(&v)
	}
	// A link to a host's page is illustrated by that host's sky; a link to
	// the empty page, by the site's own. Only a site that knows its public
	// address can name a picture: the address must be absolute.
	if v.Base != "" {
		if v.CardHost = v.Host; v.CardHost == "" && v.Name == "" {
			v.CardHost = s.selfHost()
		}
	}
	v.finish()
	render(w, status, v)
}

// selfHost is the site's own host name, from its public address.
func (s *Server) selfHost() string {
	u, err := url.Parse(s.BaseURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// showSelf puts the sky of the site's own key on the empty page. It never
// makes a visitor wait: it shows what the cache holds, however old, and
// fetches the key again in the background when that is stale or missing. The
// first visitor after a start sees the page without it.
func (s *Server) showSelf(v *view) {
	host := s.selfHost()
	if host == "" {
		return
	}
	s.mu.Lock()
	hit, ok := s.cache[host]
	s.mu.Unlock()
	if !ok || s.now().Sub(hit.at) >= cacheTTL {
		go s.RefreshSelf()
	}
	if !ok {
		return
	}
	c := stellar.New(hit.host.Key.Digest)
	v.SelfHost, v.SelfName = host, c.Name
	v.SelfSVG = template.HTML(c.SVG(640, hit.host.Key.Fingerprint()))
}

// RefreshSelf fetches the site's own key into the cache. One at a time.
func (s *Server) RefreshSelf() {
	host := s.selfHost()
	if host == "" || !s.selfBusy.CompareAndSwap(false, true) {
		return
	}
	defer s.selfBusy.Store(false)
	seen, err := s.lookup(context.Background(), host)
	if err != nil {
		return
	}
	seen.Key.Label = host
	s.mu.Lock()
	s.cache[host] = cached{seen, s.now()}
	s.mu.Unlock()
}

func (s *Server) key(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxKeyBytes)
	v := s.newView(r, "")
	status := http.StatusOK
	key, err := stellar.Parse([]byte(r.FormValue("key")))
	if err != nil {
		v.Error, status = v.S("error.key"), http.StatusUnprocessableEntity
	} else {
		v.show(key)
		v.share("")
	}
	v.finish()
	render(w, status, v)
}

// sky serves the constellation of a host as an image: /sky/example.com.svg.
func (s *Server) sky(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutSuffix(r.PathValue("file"), ".svg")
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, seen, err := s.resolve(r.Context(), name)
	if err != nil {
		// An image has no language to answer in: English.
		key, status := explain(err)
		http.Error(w, message("en", key), status)
		return
	}
	key := seen.Key
	size := 640
	if n, err := strconv.Atoi(r.URL.Query().Get("size")); err == nil {
		size = min(max(n, 64), 2048)
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	io.WriteString(w, stellar.New(key.Digest).SVG(size, key.Fingerprint()))
}

// card serves the picture a messaging application shows under a link to a
// host's page: /card/example.com.png. Drawing one takes some forty
// milliseconds, so each is drawn once per key and kept.
func (s *Server) card(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutSuffix(r.PathValue("file"), ".png")
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, seen, err := s.resolve(r.Context(), name)
	if err != nil {
		key, status := explain(err)
		http.Error(w, message("en", key), status)
		return
	}
	digest := seen.Key.Digest
	s.mu.Lock()
	picture, ok := s.cards[digest]
	s.mu.Unlock()
	if !ok {
		picture = stellar.New(digest).PNG(cardWidth, cardHeight)
		s.mu.Lock()
		if len(s.cards) >= cardsMax {
			clear(s.cards)
		}
		s.cards[digest] = picture
		s.mu.Unlock()
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write(picture)
}

// announce compares the key a host showed with the keys its owner announces.
func (v *view) announce(host string, seen stellar.Host) {
	a := seen.Announcement
	if a == nil {
		return
	}
	v.Record = stellar.Record(host, seen.Key.Digest)
	v.AnnouncedSigned = a.Signed
	switch {
	case a.Announces(seen.Key.Digest):
		v.Announced = "yes"
	case len(a.Keys) > 0:
		v.Announced = "other"
		names := make([]string, len(a.Keys))
		for i, k := range a.Keys {
			names[i] = stellar.New(k).Name
		}
		v.AnnouncedNames = strings.Join(names, ", ")
	case a.Other > 0:
		v.Announced = "unread"
	default:
		v.Announced = "none"
	}
}

func (v *view) show(key stellar.Key) {
	c := stellar.New(key.Digest)
	v.Name = c.Name
	v.Fingerprint = key.Fingerprint()
	v.Kind = key.Kind
	v.Label = key.Label
	// SVG escapes the only free text it embeds, the caption.
	v.SVG = template.HTML(c.SVG(640, key.Fingerprint()))
}

// share builds the Threema link. It is a plain link: nothing loads from
// Threema until someone clicks, and the app opens with the text written, to
// edit before sending.
func (v *view) share(baseURL string) {
	var text strings.Builder
	if v.Host != "" {
		text.WriteString(v.S("share.host", v.Host, v.Name))
	} else {
		text.WriteString(v.S("share.key", v.Name))
	}
	text.WriteString(v.Fingerprint)
	if v.Host != "" && baseURL != "" {
		text.WriteString("\n\n" + baseURL + "/?host=" + url.QueryEscape(v.Host))
	}
	// %20 rather than +, which not every client reads back as a space.
	v.ShareURL = "https://threema.id/compose?text=" + strings.ReplaceAll(url.QueryEscape(text.String()), "+", "%20")
}

func render(w http.ResponseWriter, status int, v view) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Language", v.Lang)
	w.Header().Set("Vary", "Accept-Language")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	page.Execute(w, v)
}

var errBusy = errors.New("too many lookups in progress")

// resolve validates what the visitor typed and returns what the host shows,
// from the cache when it is fresh.
func (s *Server) resolve(ctx context.Context, input string) (string, stellar.Host, error) {
	host, err := cleanHost(input)
	if err != nil {
		return "", stellar.Host{}, err
	}

	s.mu.Lock()
	hit, ok := s.cache[host]
	s.mu.Unlock()
	if ok && s.now().Sub(hit.at) < cacheTTL {
		return host, hit.host, nil
	}

	// A bounded number of outbound connections, so a burst of requests
	// cannot turn the server into a scanner.
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return "", stellar.Host{}, errBusy
	}

	ctx, cancel := context.WithTimeout(ctx, lookupBudget)
	defer cancel()
	seen, err := s.lookup(ctx, host)
	if err != nil {
		return "", stellar.Host{}, err
	}
	seen.Key.Label = host

	s.mu.Lock()
	if len(s.cache) >= cacheMax {
		clear(s.cache)
	}
	s.cache[host] = cached{seen, s.now()}
	s.mu.Unlock()
	return host, seen, nil
}

// explain turns a lookup error into the key of a sentence for the visitor and
// an HTTP status. Unknown errors are not echoed: they may describe the server's
// network.
func explain(err error) (string, int) {
	var (
		dnsErr  *net.DNSError
		certErr *tls.CertificateVerificationError
	)
	switch {
	case errors.Is(err, errBadHost):
		return "error.host", http.StatusBadRequest
	case errors.Is(err, errNotPublic):
		return "error.private", http.StatusBadRequest
	case errors.Is(err, errBusy):
		return "error.busy", http.StatusServiceUnavailable
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return "error.unknown", http.StatusNotFound
	case errors.As(err, &certErr):
		return "error.cert", http.StatusBadGateway
	case errors.Is(err, context.DeadlineExceeded), isTimeout(err):
		return "error.timeout", http.StatusGatewayTimeout
	default:
		return "error.reach", http.StatusBadGateway
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
