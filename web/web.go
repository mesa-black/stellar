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
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
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

	// Music is the address of a piece of music the pages play, and
	// MusicCredit the line that says what it is. Empty turns it off: the
	// file is read from someone else's server, which then sees the visitor's
	// address, and what may be played is for whoever runs the site to know.
	Music       string
	MusicCredit string

	lookup Lookup
	now    func() time.Time
	slots  chan struct{}
	// selfBusy is set while the site's own key is being fetched again.
	selfBusy atomic.Bool

	// drawing is held while a PNG is drawn.
	drawing sync.Mutex

	mu    sync.Mutex
	cache map[string]cached
	// cards holds the preview pictures already drawn, by key.
	cards map[drawn][]byte
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
		cards:  map[drawn][]byte{},
		logs:   map[string]cachedLogs{},
	}
}

// Handler returns the routes of the site.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("POST /key", s.key)
	mux.HandleFunc("POST /verify", s.verify)
	mux.HandleFunc("GET /sky/{file}", s.sky)
	mux.HandleFunc("GET /card/{file}", s.card)
	mux.HandleFunc("GET /chart/{file}", s.chartFile)
	mux.HandleFunc("GET /signs/{file}", signsFile)
	mux.Handle("GET /static/", static())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	return s.headers(compress(mux))
}

// headers sets the security headers. The pages carry no inline script or
// style and load nothing from elsewhere, except the music when there is one,
// so the policy can be strict. The site asks for no device and opens no
// window it needs to talk to, and says so. Started with an https address, it
// tells browsers to come back over https only, for a year, and tells it
// only to those who asked for that address: the same program reached under
// another name, a developer's own machine for one, promises nothing about
// that name.
func (s *Server) headers(next http.Handler) http.Handler {
	csp := "default-src 'none'; img-src 'self'; style-src 'self'; font-src 'self'; script-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
	if u, err := url.Parse(s.Music); err == nil && u.Scheme == "https" && u.Hostname() != "" {
		// The host of the music and the hosts under it: an archive hands a
		// download over to one of its own servers.
		csp += "; media-src https://" + u.Hostname() + " https://*." + u.Hostname()
	}
	secure := ""
	if u, err := url.Parse(s.BaseURL); err == nil && u.Scheme == "https" {
		secure = u.Host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		if secure != "" && strings.EqualFold(r.Host, secure) {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
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
	// Static is where the files of the site are, under their version.
	Static string
	// Music and MusicCredit are the server's; empty when there is none.
	Music       string
	MusicCredit string
	// Gate is the six signs of the key shown, and Origin the sky of this
	// site's own key, nil while that key is not known.
	Gate   []sign
	Origin *sign
	origin *[32]byte
	// Stardate is the moment the page was drawn, the way a ship's log would
	// write it, and StardateOf the same moment in plain words.
	Stardate   string
	StardateOf string
	// With is another host, whose key is placed on the chart beside this
	// one: its sky, how far apart the two places are, and whether the two
	// hosts serve the same key. WithQuery is what was typed and WithError
	// why it could not be read.
	With, WithQuery, WithName, WithApart, WithError string
	WithSame                                        bool
	with                                            *[32]byte
	// Verify is what came of comparing a key the visitor was given with the
	// key the host serves, VerifyError why the key given could not be read,
	// and Signs the address of the picture of the catalogue, which the
	// script spins through before it shows the result.
	Verify      *verification
	VerifyError string
	Signs       string
	digest      [32]byte
	// Chart is the place of the key among the signs, and Trip whether it
	// also shows the place this site looked from.
	Chart template.HTML
	Trip  bool
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
	v := view{Query: query, Base: s.BaseURL, Music: s.Music, MusicCredit: s.MusicCredit, Static: "/static/" + staticVersion()}
	now := s.now().UTC()
	v.Stardate, v.StardateOf = stardate(now), now.Format("2006-01-02 15:04 UTC")
	if host := s.selfHost(); host != "" {
		s.mu.Lock()
		if hit, ok := s.cache[host]; ok {
			v.origin = &hit.host.Key.Digest
		}
		s.mu.Unlock()
	}
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
		status = s.showHost(r, &v, query)
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

// showHost fills the page with what a host serves: its sky and what the
// certificate, the handshake and the DNS say beside it. It returns the
// status of the answer; a host that cannot be read leaves only the reason.
func (s *Server) showHost(r *http.Request, v *view, query string) int {
	host, seen, err := s.resolve(r.Context(), query)
	if err != nil {
		key, status := explain(err)
		v.Error = v.S(key)
		return status
	}
	v.Query, v.Host = host, host
	s.compare(r, v)
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
	return http.StatusOK
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
		// The same sky as a square picture, for the places an SVG cannot go.
		// One size, so that each is drawn once per key and kept.
		if name, ok = strings.CutSuffix(r.PathValue("file"), ".png"); ok {
			s.picture(w, r, name, mapSide, mapSide)
			return
		}
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

// compare reads the other host a visitor asked to place on the chart, with=
// in the address. It is fetched like any host; one that cannot be read
// leaves the chart as it is, and the page says why.
func (s *Server) compare(r *http.Request, v *view) {
	v.WithQuery = strings.TrimSpace(r.URL.Query().Get("with"))
	if v.WithQuery == "" {
		return
	}
	host, seen, err := s.resolve(r.Context(), v.WithQuery)
	if err != nil {
		key, _ := explain(err)
		v.WithError = v.S(key)
		return
	}
	v.With, v.WithQuery, v.with = host, host, &seen.Key.Digest
}

// chartFile serves the chart of a host as a file to keep,
// /chart/example.com.svg: the chart of the page, seen from where the page
// first shows it, with its own colours and the name of the sky under it.
func (s *Server) chartFile(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutSuffix(r.PathValue("file"), ".svg")
	if !ok {
		http.NotFound(w, r)
		return
	}
	v := s.newView(r, "")
	_, seen, err := s.resolve(r.Context(), name)
	if err != nil {
		key, status := explain(err)
		http.Error(w, message("en", key), status)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	s.compare(r, &v)
	io.WriteString(w, string(chart(seen.Key.Digest, v.origin, v.S("chart.arrival"), v.S("gate.origin"), true, v.with, v.With)))
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
	s.picture(w, r, name, cardWidth, cardHeight)
}

// picture serves the PNG of a host's sky, drawn once per key and size.
func (s *Server) picture(w http.ResponseWriter, r *http.Request, name string, width, height int) {
	_, seen, err := s.resolve(r.Context(), name)
	if err != nil {
		key, status := explain(err)
		http.Error(w, message("en", key), status)
		return
	}
	which := drawn{seen.Key.Digest, width, height}
	s.mu.Lock()
	picture, ok := s.cards[which]
	s.mu.Unlock()
	if !ok {
		// One at a time: a drawing takes some megabytes while it lasts, and
		// the site lives within a small memory limit.
		s.drawing.Lock()
		// Looked for again once the turn has come: those who waited behind
		// the one drawing this very picture take it, and do not draw it
		// again one after the other.
		s.mu.Lock()
		picture, ok = s.cards[which]
		s.mu.Unlock()
		if !ok {
			picture = stellar.New(which.digest).PNG(width, height)
			// Given back at once: left to itself the runtime keeps the
			// buffers of the last few drawings, and they add up past the
			// limit.
			debug.FreeOSMemory()
			s.mu.Lock()
			if len(s.cards) >= cardsMax {
				clear(s.cards)
			}
			s.cards[which] = picture
			s.mu.Unlock()
		}
		s.drawing.Unlock()
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

// drawn names a picture already drawn: a key at a size.
type drawn struct {
	digest        [32]byte
	width, height int
}

// stardate writes a moment as a year and the share of that year already
// gone, to two decimals: 2026.90 is nine tenths into 2026. One hundredth of
// a year is a little over three and a half days. It is a wink, not a
// calendar: nothing is computed from it.
func stardate(t time.Time) string {
	t = t.UTC()
	start := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	gone := t.Sub(start).Seconds() / start.AddDate(1, 0, 0).Sub(start).Seconds()
	// Truncated, so that the last days of December do not read as next year.
	hundredths := strconv.Itoa(int(gone * 100))
	if len(hundredths) < 2 {
		hundredths = "0" + hundredths
	}
	return strconv.Itoa(t.Year()) + "." + hundredths
}

// mapSide is the side of the square PNG of a sky, in pixels.
const mapSide = 1024

// sign is one sign of a gate, ready for the page.
type sign struct {
	Name string // what the sign is, said on hover
	Word string // what stands under it
	SVG  template.HTML
}

func (v *view) show(key stellar.Key) {
	c := stellar.New(key.Digest)
	// Numbered like the chevrons of a gate; the name of a sign is said on
	// hover. "Chevron" is the same word in the three languages.
	for i, k := range stellar.Gate(key.Digest) {
		g := stellar.Sign(k)
		v.Gate = append(v.Gate, sign{Name: g.Name, Word: "Chevron " + strconv.Itoa(i+1), SVG: template.HTML(g.GlyphSVG())})
	}
	if v.origin != nil {
		o := stellar.New(*v.origin)
		v.Origin = &sign{Name: o.Name, Word: "Chevron " + strconv.Itoa(stellar.GateSigns+1), SVG: template.HTML(o.GlyphSVG())}
	}
	v.digest = key.Digest
	v.Trip = v.origin != nil && *v.origin != key.Digest
	v.Chart = chart(key.Digest, v.origin, v.S("chart.arrival"), v.S("gate.origin"), false, v.with, v.With)
	if v.with != nil {
		v.WithName = stellar.New(*v.with).Name
		v.WithSame = *v.with == key.Digest
		v.WithApart = strconv.FormatFloat(stellar.Apart(key.Digest, *v.with), 'f', 2, 64)
		if v.Lang != "en" {
			// French and Spanish write the decimal mark as a comma.
			v.WithApart = strings.Replace(v.WithApart, ".", ",", 1)
		}
	}
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
