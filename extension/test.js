// Checks this port against the reference skies of the repository:
//   node extension/test.js            (from the root of the repository)
// It exits with an error if a single sky differs from testdata/v1.json.
"use strict";

var fs = require("fs"), path = require("path");
require("./stellar.js");

var vectors = JSON.parse(fs.readFileSync(path.join(__dirname, "..", "testdata", "v1.json"), "utf8"));
var failed = 0;
vectors.forEach(function (v) {
  var digest = Uint8Array.from(Buffer.from(v.digest, "hex"));
  var got = globalThis.Stellar.sky(digest);
  var same = got.name === v.name &&
    JSON.stringify(got.stars.map(function (s) { return [s.x, s.y, s.magnitude, s.class]; })) === JSON.stringify(v.stars) &&
    JSON.stringify(got.edges) === JSON.stringify(v.edges) &&
    JSON.stringify(got.field.map(function (s) { return [s.x, s.y, s.brightness]; })) === JSON.stringify(v.field);
  if (!same) {
    failed++;
    console.error("✗ the digest " + v.digest + " is not drawn as Stellar draws it: " + got.name + ", want " + v.name);
  }
});
if (failed) process.exit(1);
console.log("✓ " + vectors.length + " reference skies, drawn as Stellar draws them");
