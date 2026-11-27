package web

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mesa-black/stellar"
)

// The pages carry a script, and nothing above runs it: a test here reads the
// markup, not what a browser makes of it. TestBrowser opens the site in a
// real Firefox, without a window, and lets a driver act on the page as a
// visitor would: it waits, fills a form, submits it, and says what it saw.
//
// Nothing is installed for it. The driver is one more script of the page,
// served from the site's own origin so that the content security policy
// applies to it as to the rest; it reports by asking for a picture, the one
// request that policy allows. The test is skipped where there is no Firefox
// (set STELLAR_FIREFOX to the binary to say where one is).
func TestBrowser(t *testing.T) {
	firefox := findFirefox()
	if firefox == "" {
		// Where the test is the point, as in the pipeline, a missing browser
		// is a failure and not a silence.
		if os.Getenv("STELLAR_BROWSER_REQUIRED") != "" {
			t.Fatal("no Firefox, and STELLAR_BROWSER_REQUIRED is set")
		}
		t.Skip("no Firefox on this machine (STELLAR_FIREFOX=/path/to/firefox)")
	}
	if testing.Short() {
		t.Skip("opens a browser: skipped in short mode")
	}

	served := stellar.Key{Digest: sha256.Sum256([]byte("example.com"))}
	other := stellar.Key{Digest: sha256.Sum256([]byte("example.org"))}
	driver := strings.NewReplacer("%SERVED%", served.Fingerprint(), "%OTHER%", other.Fingerprint()).Replace(browserDriver)

	var calls atomic.Int32
	site := New(fakeLookup(&calls)).Handler()
	var mu sync.Mutex
	type finding struct {
		name, detail string
		ok           bool
	}
	var findings []finding
	done := make(chan struct{})
	var once sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__driver.js":
			w.Header().Set("Content-Type", "text/javascript")
			w.Write([]byte(driver))
			return
		case "/__report":
			q := r.URL.Query()
			if q.Get("done") != "" {
				once.Do(func() { close(done) })
			} else {
				mu.Lock()
				findings = append(findings, finding{q.Get("name"), q.Get("detail"), q.Get("ok") == "1"})
				mu.Unlock()
			}
			w.Header().Set("Content-Type", "image/gif")
			return
		}
		// The page as the site serves it, with the driver added at its end.
		r.Header.Del("Accept-Encoding")
		rec := httptest.NewRecorder()
		site.ServeHTTP(rec, r)
		body := rec.Body.Bytes()
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			body = []byte(strings.Replace(string(body), "</body>", `<script src="/__driver.js"></script></body>`, 1))
		}
		for name, values := range rec.Header() {
			if name != "Content-Length" {
				w.Header()[name] = values
			}
		}
		w.WriteHeader(rec.Code)
		w.Write(body)
	}))
	defer server.Close()

	profile := t.TempDir()
	prefs := `user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("datareporting.policy.dataSubmissionEnabled", false);
user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
user_pref("browser.aboutwelcome.enabled", false);
user_pref("ui.prefersReducedMotion", 0);
`
	if err := os.WriteFile(filepath.Join(profile, "user.js"), []byte(prefs), 0o600); err != nil {
		t.Fatal(err)
	}
	browser := exec.Command(firefox, "--headless", "--no-remote", "--profile", profile, server.URL+"/?host=example.com")
	if err := browser.Start(); err != nil {
		t.Fatalf("Firefox does not start: %v", err)
	}
	defer func() {
		browser.Process.Kill()
		browser.Wait()
	}()

	select {
	case <-done:
	case <-time.After(90 * time.Second):
		t.Error("the browser did not finish within 90 seconds")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, f := range findings {
		if f.ok {
			t.Logf("✓ %s", f.name)
		} else {
			t.Errorf("✗ %s (%s)", f.name, f.detail)
		}
	}
	if len(findings) < browserFindings {
		t.Errorf("%d things seen in the browser, want %d: the driver stopped on the way", len(findings), browserFindings)
	}
}

// findFirefox returns a Firefox to run, or nothing.
func findFirefox() string {
	candidates := []string{
		os.Getenv("STELLAR_FIREFOX"),
		"/Applications/Firefox.app/Contents/MacOS/firefox",
		"/Applications/Firefox Nightly.app/Contents/MacOS/firefox",
		"/Applications/Firefox Developer Edition.app/Contents/MacOS/firefox",
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	for _, name := range []string{"firefox", "firefox-esr"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// browserFindings is how many things the driver reports when it goes all
// the way.
const browserFindings = 12

// browserDriver walks the site as a visitor: the page of a host, the guard
// against a private key, a key that is the right one, a key that is not,
// and the page of the host again. It keeps its place across pages in the
// session's storage. %SERVED% and %OTHER% are two fingerprints.
const browserDriver = `(function () {
  "use strict";
  var step = sessionStorage.getItem("e2e") || "host";
  function report(name, ok, detail) {
    new Image().src = "/__report?name=" + encodeURIComponent(name) + "&ok=" + (ok ? 1 : 0) + "&detail=" + encodeURIComponent(detail || "");
  }
  // A report is a request, and leaving the page cancels the requests still
  // on their way: the page is left a moment after the last one.
  function leave(act) { setTimeout(act, 400); }
  function finish() { setTimeout(function () { new Image().src = "/__report?done=1"; }, 300); }
  function later(ms, act) {
    setTimeout(function () {
      try { act(); } catch (e) { report(step + ": the driver failed", false, String(e)); finish(); }
    }, ms);
  }
  function give(key) {
    var form = document.querySelector("#verify form");
    form.elements.key.value = key;
    form.requestSubmit();
    return form;
  }
  function signs() { return Array.prototype.slice.call(document.querySelectorAll("#verify ol li")); }
  function all(list, name) { return list.length === 6 && list.every(function (li) { return li.className === name; }); }

  window.addEventListener("load", function () {
    var status = document.getElementById("status");
    var verdict = document.querySelector("#verify .verdict");

    if (step === "host") {
      later(1200, function () {
        report("a first visit is said to be one", status && !status.hidden && status.dataset.state === "new", status && status.textContent);
        var heavens = document.querySelector(".heavens");
        var points = heavens ? heavens.querySelectorAll("b").length : 0;
        report("the background sky is sown", points >= 25, points + " points");

        var chart = document.querySelector("svg[data-yaw]");
        chart.scrollIntoView();
        later(800, function () {
          var lit = chart.querySelector("g.lit"), before = lit.getAttribute("transform");
          later(1500, function () {
            report("the chart turns while it is in sight", lit.getAttribute("transform") !== before, before + " then " + lit.getAttribute("transform"));

            var form = give("-----BEGIN OPENSSH PRIVATE KEY-----");
            var refused = form.querySelector(".refused");
            report("a private key is not sent", !refused.hidden && form.elements.key.value === "" && location.pathname === "/", refused.textContent);

            sessionStorage.setItem("e2e", "right");
            leave(function () { give("%SERVED%"); });
          });
        });
      });
    }

    if (step === "right" || step === "wrong") {
      report("the signs spin before they stop (" + step + " key)", all(signs(), "spin") && verdict.hidden, signs().map(function (li) { return li.className; }).join(","));
      later(4600, function () {
        var classes = signs().map(function (li) { return li.className; });
        if (step === "right") {
          report("the right key is confirmed", !verdict.hidden && verdict.dataset.match === "yes" && all(signs(), "ok"), verdict.dataset.match + " " + classes.join(","));
          sessionStorage.setItem("e2e", "wrong");
          leave(function () { give("%OTHER%"); });
        } else {
          report("another key is refused", !verdict.hidden && verdict.dataset.match === "no" && classes.indexOf("bad") >= 0 && classes.indexOf("spin") < 0, verdict.dataset.match + " " + classes.join(","));
          sessionStorage.setItem("e2e", "again");
          leave(function () { location.href = "/verify?host=example.com&key=" + encodeURIComponent("%SERVED%") + "&with=example.org"; });
        }
      });
    }

    if (step === "again") {
      later(4600, function () {
        report("a key given in the address is checked too", !verdict.hidden && verdict.dataset.match === "yes", verdict.dataset.match);
        report("a second visit says the sky is the same", status && status.dataset.state === "same", status && status.textContent);
        report("another host stands on the chart", !!document.querySelector("svg[data-yaw] g.other") && !!document.querySelector("#chart .note.with"), "");
        report("no private key was kept in the form", document.querySelector("#verify form").elements.key.value === "", "");
        finish();
      });
    }
  });
})();
`
