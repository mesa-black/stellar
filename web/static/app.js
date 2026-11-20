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

  function day(ms) {
    return new Date(ms).toLocaleDateString(undefined, { year: "numeric", month: "long", day: "numeric" });
  }

  function say(state, text) {
    status.dataset.state = state;
    status.textContent = text;
    status.hidden = false;
  }

  var known = load();
  if (!known || !known.fingerprint) {
    if (save(Date.now())) {
      say("new", "First time this browser sees this sky. It will remember it.");
    }
  } else if (known.fingerprint === seen.fingerprint) {
    save(known.first);
    say("same", "Same sky as on your earlier visits. First seen on " + day(known.first) + ".");
  } else {
    say("changed", "This sky has changed. On " + day(known.last) + " this site was " + known.name +
      ". A site gets a new sky when it replaces its key, which is routine at certificate renewal, " +
      "and also when someone else answers in its place.");
    accept.hidden = false;
    accept.addEventListener("click", function () {
      if (save(Date.now())) {
        accept.hidden = true;
        say("new", "Remembered. " + seen.name + " is now the sky this browser expects.");
      }
    });
  }
})();
