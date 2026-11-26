package web

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// What goes over the wire, apart from the pages themselves: how long a
// browser may keep the files of the site, and how small they travel.

// staticVersion names the files of the site by what they hold: the first
// twelve characters of a SHA-256 over their names and contents. The pages
// link to /static/<version>/…, so a file can be kept for ever by whoever
// fetched it: when one changes, the address changes with it. A stylesheet
// asks for its fonts beside itself, so the fonts follow.
var staticVersion = sync.OnceValue(func() string {
	static, _ := fs.Sub(staticFS, "static")
	var names []string
	fs.WalkDir(static, ".", func(name string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, name)
		}
		return nil
	})
	sort.Strings(names)
	sum := sha256.New()
	for _, name := range names {
		data, _ := fs.ReadFile(static, name)
		io.WriteString(sum, name+"\x00")
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))[:12]
})

// static serves the files of the site. Under the current version they never
// change; under any other address, an old version or none, the same files
// are served for an hour, so that a page kept from before a deployment still
// finds them.
func static() http.Handler {
	files, _ := fs.Sub(staticFS, "static")
	server := http.FileServerFS(files)
	current := staticVersion() + "/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/static/")
		keep := "public, max-age=3600"
		if rest, ok := strings.CutPrefix(name, current); ok {
			name, keep = rest, "public, max-age=31536000, immutable"
		} else if version, rest, ok := strings.Cut(name, "/"); ok && isVersion(version) {
			name = rest
		}
		w.Header().Set("Cache-Control", keep)
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + name
		server.ServeHTTP(w, r2)
	})
}

// isVersion tells a version from a directory of the site, such as fonts.
func isVersion(s string) bool {
	if len(s) != 12 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// A gzip writer holds more than a megabyte, and the site lives within a
// small memory limit: there are gzipAtOnce of them, made once and handed
// round. When all are busy, one more answer goes as it is, which costs bytes
// and never memory.
const gzipAtOnce = 4

var gzipWriters = func() chan *gzip.Writer {
	writers := make(chan *gzip.Writer, gzipAtOnce)
	for i := 0; i < gzipAtOnce; i++ {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		writers <- w
	}
	return writers
}()

// compress sends text gzipped to whoever accepts it: pages, styles, scripts
// and drawings in SVG, three to four times smaller. Pictures and fonts are
// compressed already and go as they are.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Range") != "" || r.URL.Path == "/healthz" ||
			!strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		g := &gzipWriter{ResponseWriter: w}
		defer g.close()
		next.ServeHTTP(g, r)
	})
}

type gzipWriter struct {
	http.ResponseWriter
	zip     *gzip.Writer
	decided bool
}

// decide looks at what is about to be sent, once: only a full answer of a
// kind that compresses is gzipped.
func (g *gzipWriter) decide(status int) {
	if g.decided {
		return
	}
	g.decided = true
	h := g.Header()
	h.Add("Vary", "Accept-Encoding")
	if status != http.StatusOK || h.Get("Content-Encoding") != "" || !compressible(h.Get("Content-Type")) {
		return
	}
	select {
	case g.zip = <-gzipWriters:
	default:
		return
	}
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	g.zip.Reset(g.ResponseWriter)
}

func (g *gzipWriter) WriteHeader(status int) {
	g.decide(status)
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipWriter) Write(p []byte) (int, error) {
	if !g.decided {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(p))
		}
		g.decide(http.StatusOK)
	}
	if g.zip != nil {
		return g.zip.Write(p)
	}
	return g.ResponseWriter.Write(p)
}

func (g *gzipWriter) close() {
	if g.zip != nil {
		g.zip.Close()
		g.zip.Reset(io.Discard)
		gzipWriters <- g.zip
	}
}

func compressible(kind string) bool {
	kind, _, _ = strings.Cut(kind, ";")
	switch strings.TrimSpace(kind) {
	case "image/svg+xml", "application/json", "application/javascript":
		return true
	}
	return strings.HasPrefix(kind, "text/")
}
