// Stellar for Firefox: the key of the page in front of you, as your own
// machine received it.
//
// The website stellar.mesa.black shows the key a site presented to the
// Stellar server. Somebody intercepting your connection is not visible from
// there. This reads the certificate of the page in each tab from the
// browser itself, and keeps the SHA-256 of its public key, which is the
// digest a sky is drawn from.
//
// Nothing leaves the machine: no request is made, nothing is logged. What a
// tab showed is kept for the session, to answer the popup; what a site
// showed last time is kept in this browser, to say when it changed.
"use strict";

// The digest Firefox gives is base64; a sky is drawn from its 32 bytes,
// kept here in hexadecimal.
function hex(base64) {
  var binary = atob(base64), out = "";
  for (var i = 0; i < binary.length; i++) out += binary.charCodeAt(i).toString(16).padStart(2, "0");
  return out;
}

async function seen(details) {
  try {
    var info = await browser.webRequest.getSecurityInfo(details.requestId, {});
    var entry = { host: new URL(details.url).host, state: info.state };
    var certificate = info.certificates && info.certificates[0];
    if (certificate) {
      entry.digest = hex(certificate.subjectPublicKeyInfoDigest.sha256);
      entry.issuer = certificate.issuer;
      entry.until = certificate.validity.end;
      entry.protocol = info.protocolVersion;
      entry.exchange = info.keaGroupName;
      entry.untrusted = !!(info.isUntrusted || info.isNotValidAtThisTime || info.isDomainMismatch);
    }
    await browser.storage.session.set({ ["tab:" + details.tabId]: entry });
    await mark(details.tabId, entry);
  } catch (e) {
    // A page whose certificate cannot be read is shown as such by the popup.
  }
}

// A badge on the button when the key of a site is not the one this browser
// remembers for it: the popup says what changed.
async function mark(tabId, entry) {
  if (!entry.digest) return;
  var key = "host:" + entry.host;
  var known = (await browser.storage.local.get(key))[key];
  var changed = !!known && known.digest !== entry.digest;
  await browser.action.setBadgeText({ tabId: tabId, text: changed ? "!" : "" });
  if (changed) await browser.action.setBadgeBackgroundColor({ tabId: tabId, color: "#ffb070" });
}

// The certificate can only be asked for while the answer is being received,
// from a listener that may hold the request: hence "blocking". The request
// itself is never changed.
browser.webRequest.onHeadersReceived.addListener(
  function (details) { seen(details); },
  { urls: ["https://*/*"], types: ["main_frame"] },
  ["blocking"]
);

browser.tabs.onRemoved.addListener(function (tabId) {
  browser.storage.session.remove("tab:" + tabId);
});
