# Stellar for Firefox

The key of the page in front of you, drawn as a constellation, **as your own
machine received it**.

The website [stellar.mesa.black](https://stellar.mesa.black) shows the key a
site presented to the Stellar server. Somebody intercepting *your*
connection is not visible from there. This extension reads the certificate
of the page in each tab from the browser itself, and draws the sky of its
public key: the same sky the website draws for the same key.

## What it does

- **The sky of this page.** The button opens the constellation of the key
  the site presented to your browser, with its name, its fingerprint, who
  issued the certificate and until when, and whether the key exchange was a
  post-quantum hybrid.
- **Whether it changed.** The browser remembers the sky each site showed
  last time, and says when it is not the same; the button then carries a
  mark. A renewed certificate changes the sky, and so does somebody in the
  way: the popup says which sky was expected, and lets you accept the new
  one.
- **Whether the server sees the same.** One button asks stellar.mesa.black
  to compare the key seen from your machine with the one its server
  receives from the same site. Two different keys is what an interception
  looks like.

## What leaves your machine

Nothing, until you press "Compare with what the server sees". That button
sends the name of the site and the fingerprint of its public key to
stellar.mesa.black, in a new tab, and nothing else. A public key is public;
the name of the site you are on is what you are telling the server.

What each site showed last time is kept in this browser's storage for the
extension, and nowhere else.

## Permissions

- **Access to your data for all websites** (`https://*/*`) and **web
  requests**: a certificate can only be read while a page arrives, from a
  listener on the request. The request is never changed, and no page
  content is read.
- **Storage**: the sky each site showed last time.

## Why Firefox only

Firefox is the one browser that lets an extension read the certificate of a
page (`webRequest.getSecurityInfo`). Chrome and Safari do not.

## Trying it

It is not published on addons.mozilla.org yet. To load it for a session:
open `about:debugging#/runtime/this-firefox`, press "Load Temporary
Add-on…" and choose `manifest.json` in this directory. Then reload the page
you want to look at: a key is read while a page arrives.

`make extension`, at the root of the repository, packages it into
`bin/stellar-extension.zip`.

## The mapping

`stellar.js` is a port of version 1 of the mapping, the third after Go and
PHP. `node extension/test.js` checks it against the 32 reference skies of
`testdata/v1.json`, and the pipeline runs it on every push.
