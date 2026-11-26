// Stellar, version 1 of the mapping, for a browser: a digest in, a sky out.
//
// This is the third implementation of the mapping, after Go (the reference)
// and PHP (Sablier). testdata/v1.json at the root of the repository holds
// the reference skies, and extension/test.js refuses a port that draws one
// of them differently: a sky that depended on which program drew it would
// be worse than none. docs/spec.md is the description.
//
// A browser has no SHAKE256, so the sponge is written out: the plain
// Keccak-f[1600] of FIPS 202, on 64-bit lanes held as BigInt.
(function (root) {
  "use strict";

  var GRID = 64, FIELD_GRID = 256, MIN_DISTANCE_SQUARED = 81;
  var CONSONANTS = "bdfghjklmnprstvz", VOWELS = "aeiou";
  var NOUN_ENDINGS = ["a", "us", "um", "is", "ix", "or", "ax", "ia"];
  var EPITHET_ENDINGS = ["alis", "aris", "ensis", "inus", "ianus", "osus", "atus", "icus"];

  var RATE = 136;
  var MASK = (1n << 64n) - 1n;
  var ROTATIONS = [1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14, 27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44];
  var LANES = [10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4, 15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1];
  var ROUNDS = [
    0x0000000000000001n, 0x0000000000008082n, 0x800000000000808An, 0x8000000080008000n,
    0x000000000000808Bn, 0x0000000080000001n, 0x8000000080008081n, 0x8000000000008009n,
    0x000000000000008An, 0x0000000000000088n, 0x0000000080008009n, 0x000000008000000An,
    0x000000008000808Bn, 0x800000000000008Bn, 0x8000000000008089n, 0x8000000000008003n,
    0x8000000000008002n, 0x8000000000000080n, 0x000000000000800An, 0x800000008000000An,
    0x8000000080008081n, 0x8000000000008080n, 0x0000000080000001n, 0x8000000080008008n
  ];

  function rotate(lane, by) {
    return ((lane << BigInt(by)) | (lane >> BigInt(64 - by))) & MASK;
  }

  // Keccak-f[1600]: twenty-four rounds of theta, rho, pi, chi and iota.
  function permute(s) {
    for (var round = 0; round < 24; round++) {
      var c = [], x, y;
      for (x = 0; x < 5; x++) c[x] = s[x] ^ s[x + 5] ^ s[x + 10] ^ s[x + 15] ^ s[x + 20];
      for (x = 0; x < 5; x++) {
        var d = c[(x + 4) % 5] ^ rotate(c[(x + 1) % 5], 1);
        for (y = 0; y < 25; y += 5) s[x + y] ^= d;
      }
      var carried = s[1];
      for (var i = 0; i < 24; i++) {
        var next = s[LANES[i]];
        s[LANES[i]] = rotate(carried, ROTATIONS[i]);
        carried = next;
      }
      for (y = 0; y < 25; y += 5) {
        var row = s.slice(y, y + 5);
        for (x = 0; x < 5; x++) s[y + x] = row[x] ^ (~row[(x + 1) % 5] & MASK & row[(x + 2) % 5]);
      }
      s[0] ^= ROUNDS[round];
    }
  }

  // A stream of integers drawn from SHAKE256(domain ‖ digest).
  function Stream(domain, digest) {
    var message = [];
    for (var i = 0; i < domain.length; i++) message.push(domain.charCodeAt(i));
    for (i = 0; i < digest.length; i++) message.push(digest[i]);
    message.push(0x1F);
    while (message.length % RATE !== 0) message.push(0);
    message[message.length - 1] |= 0x80;

    this.state = [];
    for (i = 0; i < 25; i++) this.state.push(0n);
    for (var offset = 0; offset < message.length; offset += RATE) {
      for (var lane = 0; lane < RATE / 8; lane++) {
        var value = 0n;
        for (var b = 7; b >= 0; b--) value = (value << 8n) | BigInt(message[offset + lane * 8 + b]);
        this.state[lane] ^= value;
      }
      permute(this.state);
    }
    this.squeezed = [];
  }

  // A uniform integer in [0, n), for n up to 65536: two bytes read
  // big-endian, drawn again when they would bias the result.
  Stream.prototype.draw = function (n) {
    var limit = 65536 - 65536 % n, value;
    do {
      if (this.squeezed.length < 2) {
        for (var lane = 0; lane < RATE / 8; lane++) {
          var v = this.state[lane];
          for (var b = 0; b < 8; b++) {
            this.squeezed.push(Number(v & 0xFFn));
            v >>= 8n;
          }
        }
        permute(this.state);
      }
      value = (this.squeezed[0] << 8) | this.squeezed[1];
      this.squeezed.splice(0, 2);
    } while (value >= limit);
    return value % n;
  };

  // The candidates, nearest to star i first; at equal distance, the lower
  // index first. The tie-break is part of the mapping.
  function nearest(stars, i, candidates) {
    function distance(j) {
      return Math.pow(stars[i].x - stars[j].x, 2) + Math.pow(stars[i].y - stars[j].y, 2);
    }
    return candidates.slice().sort(function (a, b) { return distance(a) - distance(b) || a - b; });
  }

  function capital(word) { return word[0].toUpperCase() + word.slice(1); }

  // sky returns the constellation of a digest, 32 bytes: its name, its main
  // stars, the edges between them and the field of fainter stars behind.
  // The order of the draws is the mapping.
  function sky(digest) {
    if (digest.length !== 32) throw new Error("A sky is drawn from the 32 bytes of a SHA-256.");
    var s = new Stream("stellar/v1\0", digest);
    function consonant() { return CONSONANTS[s.draw(16)]; }
    function vowel() { return VOWELS[s.draw(5)]; }

    var noun = consonant() + vowel() + consonant() + vowel() + consonant() + NOUN_ENDINGS[s.draw(8)];
    var epithet = consonant() + vowel() + consonant() + EPITHET_ENDINGS[s.draw(8)];
    var name = capital(noun) + " " + capital(epithet);

    var count = 7 + s.draw(6), stars = [];
    placing: while (stars.length < count) {
      var x = s.draw(GRID), y = s.draw(GRID);
      for (var k = 0; k < stars.length; k++) {
        if (Math.pow(stars[k].x - x, 2) + Math.pow(stars[k].y - y, 2) < MIN_DISTANCE_SQUARED) continue placing;
      }
      stars.push({ x: x, y: y, magnitude: s.draw(3), class: s.draw(5) });
    }

    // Each star joins one of the two nearest stars placed before it.
    var edges = [], joined = {};
    function link(a, b) {
      joined[a + "-" + b] = joined[b + "-" + a] = true;
      edges.push([Math.min(a, b), Math.max(a, b)]);
    }
    for (var i = 1; i < count; i++) {
      var before = [];
      for (var j = 0; j < i; j++) before.push(j);
      var near = nearest(stars, i, before);
      link(near[near.length > 1 ? s.draw(2) : 0], i);
    }
    // Up to two more edges close loops.
    for (var extra = s.draw(3); extra > 0; extra--) {
      var a = s.draw(count), free = [];
      for (j = 0; j < count; j++) if (j !== a && !joined[a + "-" + j]) free.push(j);
      if (free.length) link(a, nearest(stars, a, free)[0]);
    }

    var field = [];
    for (var n = 40 + s.draw(24); n > 0; n--) {
      field.push({ x: s.draw(FIELD_GRID), y: s.draw(FIELD_GRID), brightness: s.draw(3) });
    }
    return { name: name, stars: stars, edges: edges, field: field };
  }

  // The fingerprint of a digest, as OpenSSH prints one: base64 without
  // padding.
  function fingerprint(digest) {
    var binary = "";
    for (var i = 0; i < digest.length; i++) binary += String.fromCharCode(digest[i]);
    return "SHA256:" + btoa(binary).replace(/=+$/, "");
  }

  // How a sky is drawn: a glyph in one ink, a hollow stroke from star to
  // star that swells into a round cell at each star. The same picture as
  // the website's, built node by node since a page of an extension writes
  // no markup from a string.
  var INK = "#e8ecff", STROKE = 15, WALL = 4.2, RADII = [23, 18, 13.5];
  var SVG = "http://www.w3.org/2000/svg";

  function place(star) {
    return [110 + (star.x + 0.5) / GRID * 780, 70 + (star.y + 0.5) / GRID * 780];
  }

  function draw(document, constellation, caption) {
    function node(name, attributes, parent) {
      var el = document.createElementNS(SVG, name);
      for (var key in attributes) el.setAttribute(key, attributes[key]);
      if (parent) parent.appendChild(el);
      return el;
    }
    function fixed(n) { return n.toFixed(1); }

    var svg = node("svg", { viewBox: "0 0 1000 1000", role: "img", "aria-label": constellation.name });
    var gradient = node("radialGradient", { id: "night", gradientUnits: "userSpaceOnUse", cx: 500, cy: 420, r: 750 }, node("defs", {}, svg));
    node("stop", { offset: 0, "stop-color": "#16204a" }, gradient);
    node("stop", { offset: 1, "stop-color": "#05060f" }, gradient);
    node("rect", { width: 1000, height: 1000, rx: 4, fill: "url(#night)" }, svg);

    var field = node("g", { fill: "#cfd8ff" }, svg);
    constellation.field.forEach(function (star) {
      node("circle", {
        cx: fixed((star.x + 0.5) / FIELD_GRID * 1000), cy: fixed((star.y + 0.5) / FIELD_GRID * 1000),
        r: fixed(0.9 + 0.4 * star.brightness), opacity: (0.14 + 0.1 * star.brightness).toFixed(2)
      }, field);
    });

    // The whole figure in ink, then its inside in night: what is left is
    // the outline of one shape.
    [[INK, 0], ["url(#night)", WALL]].forEach(function (layer) {
      var g = node("g", { fill: layer[0], stroke: layer[0], "stroke-width": fixed(STROKE - 2 * layer[1]), "stroke-linecap": "round" }, svg);
      constellation.edges.forEach(function (edge) {
        var a = place(constellation.stars[edge[0]]), b = place(constellation.stars[edge[1]]);
        node("line", { x1: fixed(a[0]), y1: fixed(a[1]), x2: fixed(b[0]), y2: fixed(b[1]) }, g);
      });
      constellation.stars.forEach(function (star) {
        var at = place(star);
        node("circle", { cx: fixed(at[0]), cy: fixed(at[1]), r: fixed(RADII[star.magnitude] - layer[1]), stroke: "none" }, g);
      });
    });

    // The class of a star is the mark in its cell: none, a point, a full
    // cell, a bar, a cross.
    var marks = node("g", { fill: INK, stroke: INK, "stroke-width": WALL, "stroke-linecap": "round" }, svg);
    constellation.stars.forEach(function (star) {
      var at = place(star), x = at[0], y = at[1], r = RADII[star.magnitude] - WALL;
      if (star.class === 1) node("circle", { cx: fixed(x), cy: fixed(y), r: fixed(r * 0.36), stroke: "none" }, marks);
      if (star.class === 2) node("circle", { cx: fixed(x), cy: fixed(y), r: fixed(r), stroke: "none" }, marks);
      if (star.class === 3) node("path", { d: "M" + fixed(x - r) + " " + fixed(y) + "H" + fixed(x + r) }, marks);
      if (star.class === 4) node("path", { d: "M" + fixed(x - r) + " " + fixed(y) + "H" + fixed(x + r) + "M" + fixed(x) + " " + fixed(y - r) + "V" + fixed(y + r) }, marks);
    });

    node("text", {
      x: 500, y: 935, "text-anchor": "middle", "font-family": "ui-sans-serif, system-ui, sans-serif",
      "font-size": 34, "letter-spacing": 10, fill: INK
    }, svg).textContent = constellation.name.toUpperCase();
    if (caption) {
      node("text", {
        x: 500, y: 972, "text-anchor": "middle", "font-family": "ui-monospace, SFMono-Regular, Menlo, monospace",
        "font-size": 17, fill: INK, "fill-opacity": 0.6
      }, svg).textContent = caption;
    }
    return svg;
  }

  root.Stellar = { sky: sky, fingerprint: fingerprint, draw: draw };
})(typeof globalThis !== "undefined" ? globalThis : this);
