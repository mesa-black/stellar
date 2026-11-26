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

// The sky behind the page: faint still points, and shooting stars, one thin
// streak now and then, a first one
// soon after the page opens and then about one a minute. None for a
// visitor who asked their system for less motion, none while the tab is hidden.
(function () {
  "use strict";

  if (!document.body.animate || !window.matchMedia) return;
  var still = matchMedia("(prefers-reduced-motion: reduce)");

  var heavens = document.createElement("div");
  heavens.className = "heavens";
  heavens.setAttribute("aria-hidden", "true");
  document.body.appendChild(heavens);

  function between(a, b) { return a + Math.random() * (b - a); }

  // A few very faint points, still, sown afresh on each page: as many as the
  // window is large, most of them a single pixel.
  var points = Math.max(25, Math.min(90, Math.round(innerWidth * innerHeight / 22000)));
  for (var i = 0; i < points; i++) {
    var point = document.createElement("b");
    var size = Math.random() < 0.8 ? 1 : between(1.4, 2);
    point.style.width = point.style.height = size + "px";
    point.style.left = between(0, 100) + "%";
    point.style.top = between(0, 100) + "%";
    point.style.opacity = between(0.12, 0.55);
    heavens.appendChild(point);
  }

  function fall() {
    if (document.hidden || still.matches) return;
    // Downwards, to the left or to the right, from somewhere in the upper sky.
    var angle = between(18, 38);
    if (Math.random() < 0.5) angle = 180 - angle;
    var at = "translate(" + between(0.1, 0.9) * innerWidth + "px," + between(0.02, 0.45) * innerHeight + "px) rotate(" + angle + "deg) ";
    var star = document.createElement("i");
    star.style.width = between(90, 160) + "px";
    heavens.appendChild(star);
    star.animate([
      { transform: at + "translateX(0)", opacity: 0 },
      { opacity: 0.7, offset: 0.25 },
      { transform: at + "translateX(" + between(260, 420) + "px)", opacity: 0 }
    ], { duration: between(700, 1100), easing: "ease-in" }).onfinish = function () { star.remove(); };
  }

  function next(seconds) {
    setTimeout(function () { fall(); next(between(50, 80)); }, seconds * 1000);
  }
  next(between(10, 20));
})();

// The chart turns slowly on itself, and under the finger or the mouse. The
// page drew it once, from a fixed angle; each point carries its place in
// space, and this script projects it again, the way web/chart.go does.
(function () {
  "use strict";

  var chart = document.querySelector("svg[data-yaw]");
  if (!chart || !window.requestAnimationFrame) return;

  var RADIUS = 100, EYE = 5;
  var yaw = Number(chart.dataset.yaw), pitch = Number(chart.dataset.pitch);

  function place(text) { return text.split(" ").map(Number); }

  var points = Array.prototype.map.call(chart.querySelectorAll("[data-p]"), function (el) {
    return { el: el, p: place(el.dataset.p) };
  });
  var lines = Array.prototype.map.call(chart.querySelectorAll("[data-pts]"), function (el) {
    return { el: el, pts: el.dataset.pts.split(";").map(place) };
  });

  function draw() {
    var cy = Math.cos(yaw), sy = Math.sin(yaw), cp = Math.cos(pitch), sp = Math.sin(pitch);
    function project(p) {
      var x = p[0] * cy + p[2] * sy, z = -p[0] * sy + p[2] * cy;
      var y = p[1] * cp - z * sp;
      z = p[1] * sp + z * cp;
      var scale = RADIUS * EYE / (EYE - z);
      return [x * scale, -y * scale, z];
    }
    points.forEach(function (point) {
      var at = project(point.p);
      point.el.setAttribute("transform", "translate(" + at[0].toFixed(1) + " " + at[1].toFixed(1) + ")");
      point.el.setAttribute("opacity", (0.35 + 0.65 * (at[2] + 1) / 2).toFixed(2));
    });
    lines.forEach(function (line) {
      line.el.setAttribute("points", line.pts.map(function (p) {
        var at = project(p);
        return at[0].toFixed(1) + "," + at[1].toFixed(1);
      }).join(" "));
    });
  }

  // Held: the chart follows the pointer, and rests a moment once let go.
  var held = null, rest = 0;
  chart.addEventListener("pointerdown", function (event) {
    held = { x: event.clientX, y: event.clientY };
    chart.classList.add("held");
    try { chart.setPointerCapture(event.pointerId); } catch (e) {}
  });
  chart.addEventListener("pointermove", function (event) {
    if (!held) return;
    yaw += (event.clientX - held.x) * 0.01;
    pitch = Math.max(-1.3, Math.min(1.3, pitch + (event.clientY - held.y) * 0.01));
    held = { x: event.clientX, y: event.clientY };
    draw();
  });
  function release() {
    held = null;
    rest = 3000;
    chart.classList.remove("held");
  }
  chart.addEventListener("pointerup", release);
  chart.addEventListener("pointercancel", release);

  // One turn in two minutes; none for a visitor who asked for less motion.
  // The chart is redrawn some twenty-five times a second, which is plenty for
  // so slow a turn, and not at all while it is out of sight: off the screen,
  // in a hidden tab, or still.
  var still = window.matchMedia ? matchMedia("(prefers-reduced-motion: reduce)") : { matches: false };
  var seen = true, turning = false, last = 0, owed = 0;

  function turn(now) {
    if (!seen || document.hidden || still.matches) {
      turning = false;
      return;
    }
    var elapsed = last ? Math.min(now - last, 100) : 0;
    last = now;
    if (!held) {
      if (rest > 0) {
        rest -= elapsed;
      } else {
        owed += elapsed;
        if (owed >= 40) {
          yaw += owed * 2 * Math.PI / 120000;
          owed = 0;
          draw();
        }
      }
    }
    requestAnimationFrame(turn);
  }

  function wake() {
    if (turning) return;
    turning = true;
    last = 0;
    requestAnimationFrame(turn);
  }

  if (window.IntersectionObserver) {
    new IntersectionObserver(function (entries) {
      seen = entries[entries.length - 1].isIntersecting;
      if (seen) wake();
    }).observe(chart);
  }
  document.addEventListener("visibilitychange", wake);
  if (still.addEventListener) still.addEventListener("change", wake);
  wake();
})();

// A private key never leaves the page: a form that takes a key refuses one
// before it is sent, and says why.
(function () {
  "use strict";

  Array.prototype.forEach.call(document.querySelectorAll("form[data-private]"), function (form) {
    var refused = form.querySelector(".refused");
    form.addEventListener("submit", function (event) {
      if (form.elements.key.value.indexOf("PRIVATE KEY") < 0) return;
      event.preventDefault();
      form.elements.key.value = "";
      refused.textContent = form.dataset.private;
      refused.hidden = false;
    });
  });
})();

// The result of a check, shown the way a gate is dialled: each of the six
// signs spins through the catalogue and stops, one after the other, on the
// sign of the key that was given; then the verdict. The page already holds
// the whole result, this only delays its showing: without the script, or
// for a visitor who asked for less motion, it is simply there.
(function () {
  "use strict";

  var verify = document.getElementById("verify");
  if (!verify || !verify.dataset.gate) return;
  if (window.matchMedia && matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  var SIDE = 8, TURN = 70, FIRST = 900, EACH = 550;
  var places = Array.prototype.slice.call(verify.querySelectorAll("ol li"));
  var faces = places.map(function (place) { return place.querySelector(".face"); });
  var verdict = verify.querySelector(".verdict");
  var picture = 'url("' + verify.dataset.signs + '")';

  function show(face, k) {
    face.style.backgroundPosition = (k % SIDE) / (SIDE - 1) * 100 + "% " + Math.floor(k / SIDE) / (SIDE - 1) * 100 + "%";
  }

  // What each place will show once stopped is kept aside, and given back.
  var held = places.map(function (place, i) {
    var was = place.className;
    place.className = "spin";
    faces[i].style.backgroundImage = picture;
    return was;
  });
  verdict.hidden = true;

  var spinning = places.length;
  var turning = setInterval(function () {
    faces.forEach(function (face, i) {
      if (i >= faces.length - spinning) show(face, Math.floor(Math.random() * SIDE * SIDE));
    });
  }, TURN);

  places.forEach(function (place, i) {
    setTimeout(function () {
      faces[i].style.backgroundImage = "";
      place.className = held[i];
      spinning--;
      if (spinning === 0) {
        clearInterval(turning);
        verdict.hidden = false;
      }
    }, FIRST + i * EACH);
  });
})();
