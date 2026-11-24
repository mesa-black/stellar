// Remembers, in this browser only, the sky each host showed last time, and
// says whether today's is the same. Nothing is sent anywhere.
(function () {
  "use strict";

  var result = document.getElementById("result");
  if (!result || !result.dataset.host) return;

  var status = document.getElementById("status");
  var accept = document.getElementById("accept");
  var storageKey = "stellar:host:" + result.dataset.host;
  var seen = { name: result.dataset.name, fingerprint: result.dataset.fingerprint };

  function load() {
    try {
      return JSON.parse(localStorage.getItem(storageKey));
    } catch (e) {
      return null;
    }
  }

  function save(first) {
    try {
      localStorage.setItem(storageKey, JSON.stringify({
        name: seen.name,
        fingerprint: seen.fingerprint,
        first: first,
        last: Date.now()
      }));
      return true;
    } catch (e) {
      return false;
    }
  }

  // The sentences come from the page, in its language, with {date} and
  // {name} left for this script to fill.
  var words = result.dataset;
  var language = document.documentElement.lang || undefined;

  function day(ms) {
    return new Date(ms).toLocaleDateString(language, { year: "numeric", month: "long", day: "numeric" });
  }

  function fill(sentence, values) {
    return (sentence || "").replace(/\{(date|name)\}/g, function (_, key) { return values[key]; });
  }

  function say(state, text) {
    status.dataset.state = state;
    status.textContent = text;
    status.hidden = false;
  }

  var known = load();
  if (!known || !known.fingerprint) {
    if (save(Date.now())) {
      say("new", words.new);
    }
  } else if (known.fingerprint === seen.fingerprint) {
    save(known.first);
    say("same", fill(words.same, { date: day(known.first) }));
  } else {
    say("changed", fill(words.changed, { date: day(known.last), name: known.name }));
    accept.hidden = false;
    accept.addEventListener("click", function () {
      if (save(Date.now())) {
        accept.hidden = true;
        say("new", fill(words.accepted, { name: seen.name }));
      }
    });
  }
})();

// The music, when the site has one: on by default, cut by its button. No
// browser lets a page play sound before the visitor's first gesture, so it
// starts then. The file is asked for only at that moment: a visitor who cut
// the sound, here or on an earlier visit, never asks its server for anything.
(function () {
  "use strict";

  var button = document.getElementById("sound");
  if (!button) return;

  var words = button.dataset;
  var audio = null;
  var muted = false;
  try { muted = localStorage.getItem("stellar:sound") === "off"; } catch (e) {}

  function remember() {
    try { localStorage.setItem("stellar:sound", muted ? "off" : "on"); } catch (e) {}
  }

  function forget() {
    removeEventListener("pointerdown", first, true);
    removeEventListener("keydown", first, true);
  }

  function start() {
    if (muted) return;
    if (!audio) {
      audio = new Audio(words.src);
      audio.loop = true;
      audio.volume = 0.6;
      // Going from one page to the next, the music takes up where it was.
      try { audio.currentTime = Number(sessionStorage.getItem("stellar:sound:at")) || 0; } catch (e) {}
    }
    audio.play().then(function () {
      button.setAttribute("aria-pressed", "true");
      button.textContent = words.cut;
      forget();
    }).catch(function () {});
  }

  function stop() {
    if (audio) audio.pause();
    button.setAttribute("aria-pressed", "false");
    button.textContent = words.back;
  }

  function first() { forget(); start(); }

  button.textContent = muted ? words.back : words.first;
  button.hidden = false;
  button.addEventListener("click", function (event) {
    event.stopPropagation();
    muted = !!audio && !audio.paused;
    remember();
    if (muted) { stop(); } else { start(); }
  });
  addEventListener("pagehide", function () {
    if (!audio) return;
    try { sessionStorage.setItem("stellar:sound:at", String(audio.currentTime)); } catch (e) {}
  });

  if (!muted) {
    addEventListener("pointerdown", first, true);
    addEventListener("keydown", first, true);
    // Tried at once: a browser that already saw the visitor act on the site
    // lets it play.
    start();
  }
})();
