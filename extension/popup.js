// The popup: the sky of the key the page in this tab presented to this
// browser, whether it is the one this browser saw there last time, and a
// way to ask stellar.mesa.black whether its server sees the same key.
//
// That last question is the point of the extension. Two different keys for
// one site, one seen from here and one seen from there, is what somebody
// standing between you and the site looks like. It is asked only when the
// button is pressed: until then nothing leaves this machine.
//
// This file is a module, read from top to bottom: the page is not finished
// loading until it has drawn what it has to draw.

var t = browser.i18n.getMessage;
function $(id) { return document.getElementById(id); }
function bytes(hex) {
  var out = [];
  for (var i = 0; i < hex.length; i += 2) out.push(parseInt(hex.substr(i, 2), 16));
  return out;
}

var tabs = await browser.tabs.query({ active: true, currentWindow: true });
var tab = tabs[0];
var entry = tab && (await browser.storage.session.get("tab:" + tab.id))["tab:" + tab.id];

if (!entry || !entry.digest) {
  // Not an https page, or one loaded before the extension was: a page is
  // read while it arrives, not afterwards.
  var everywhere = { origins: ["https://*/*"] };
  if (!(await browser.permissions.contains(everywhere))) {
    // Firefox may install an extension without the access it asks for, and
    // leave it to be given here. Without it no certificate can be read.
    $("none-text").textContent = t("noneGrant");
    $("grant").textContent = t("grant");
    $("grant").hidden = false;
    $("grant").addEventListener("click", async function () {
      if (await browser.permissions.request(everywhere)) {
        $("grant").hidden = true;
        $("none-text").textContent = t("noneReload");
      }
    });
  } else {
    $("none-text").textContent = t(tab && /^https:/.test(tab.url || "") ? "noneReload" : "noneNotHttps");
  }
  $("none").hidden = false;
} else {
  var digest = bytes(entry.digest);
  var constellation = Stellar.sky(digest);
  var print = Stellar.fingerprint(digest);

  $("sky").appendChild(Stellar.draw(document, constellation, print));
  $("name").textContent = constellation.name;
  $("label-site").textContent = t("labelSite");
  $("label-print").textContent = t("labelFingerprint");
  $("label-certificate").textContent = t("labelCertificate");
  $("label-connection").textContent = t("labelConnection");
  $("host").textContent = entry.host;
  $("print").textContent = print;
  // The issuer comes as a distinguished name; its organisation and its name
  // are what a reader recognises.
  var issuer = ["O", "CN"].map(function (part) {
    var found = new RegExp("(?:^|,)" + part + "=((?:[^,\\\\]|\\\\.)*)").exec(entry.issuer || "");
    return found && found[1].replace(/\\(.)/g, "$1");
  }).filter(Boolean).join(" ") || entry.issuer || "?";
  $("certificate").textContent = t("certificate", [issuer, new Date(entry.until).toLocaleDateString()]) +
    (entry.untrusted ? " " + t("untrusted") : "");
  // The hybrid key exchanges all carry ML-KEM in their name.
  var hybrid = /mlkem|kyber/i.test(entry.exchange || "");
  $("connection").textContent = [entry.protocol, entry.exchange, t(hybrid ? "postQuantum" : "notPostQuantum")]
    .filter(Boolean).join(", ");

  // What this browser saw at this site last time. Kept here and nowhere
  // else, under the name of the site.
  var key = "host:" + entry.host;
  var known = (await browser.storage.local.get(key))[key];
  var status = $("status"), accept = $("accept");
  function say(state, text) { status.dataset.state = state; status.textContent = text; }
  function day(ms) { return new Date(ms).toLocaleDateString(undefined, { year: "numeric", month: "long", day: "numeric" }); }
  async function remember(first) {
    await browser.storage.local.set({ [key]: { digest: entry.digest, name: constellation.name, first: first, last: Date.now() } });
    await browser.action.setBadgeText({ tabId: tab.id, text: "" });
  }

  if (!known) {
    await remember(Date.now());
    say("new", t("seenNew"));
  } else if (known.digest === entry.digest) {
    await remember(known.first);
    say("same", t("seenSame", [day(known.first)]));
  } else {
    say("changed", t("seenChanged", [day(known.last), known.name]));
    accept.textContent = t("accept");
    accept.hidden = false;
    accept.addEventListener("click", async function () {
      await remember(Date.now());
      accept.hidden = true;
      say("new", t("seenAccepted", [constellation.name]));
    });
  }

  // The question to the server: the site's own check, asked with the key
  // seen from here, in a new tab where the answer is drawn. It tells the
  // server which site you are on, which is why the extension declares that
  // it transmits browsing activity: this, on this click, and nothing else.
  //
  // Firefox is not asked for that agreement here: a question put from this
  // window closes it, and what was to follow never runs.
  $("check-text").textContent = t("checkText");
  $("check-go").textContent = t("checkGo");
  // A site on another port than 443 cannot be asked about: the server only
  // reads the standard one.
  if (/:\d+$/.test(entry.host) && !/:443$/.test(entry.host)) $("check").hidden = true;
  $("check-go").addEventListener("click", async function () {
    var ask = new URL("https://stellar.mesa.black/verify");
    ask.searchParams.set("host", entry.host.replace(/:443$/, ""));
    ask.searchParams.set("key", print);
    ask.searchParams.set("lang", (browser.i18n.getUILanguage() || "en").slice(0, 2));
    ask.hash = "verify";
    await browser.tabs.create({ url: ask.href });
    window.close();
  });

  $("shown").hidden = false;
}
