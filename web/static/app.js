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
