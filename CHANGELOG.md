# Changelog

What each release of the command line brought. The website follows `main`
and may be ahead of the last entry: what it has gained since is under
*Unreleased*.

## Unreleased

Nothing yet.

## v0.5.0 — 2026-11-27

Two keys, side by side.

- **Check a key.** Under the gate of a host, a visitor pastes the key they
  were given by another road, and the site says whether it is the key the
  host serves, from the two digests compared whole. Its six signs light up
  one by one before the verdict. A private key is refused before it is read.
- **Two hosts on one chart.** `?host=a&with=b` on the website and
  `stellar -with <key>` on the command line place a second key on the chart
  and say how far apart the two stand.
- **The chart as a file.** `/chart/example.com.svg` on the website,
  `stellar -chart file.svg` on the command line.
- **`Place:`** on the command line: where the gate of a key leads, as three
  coordinates.
- The chart ties the arrival to its three lines, and has a section of its
  own on the page.
- The website keeps its files in the browser for a year, sends text
  gzipped, and sends `Strict-Transport-Security`, `Permissions-Policy` and
  `Cross-Origin-Opener-Policy`.

## v0.4.0 — 2026-11-26

A gate, a chart, and one ink.

The sky is drawn as a glyph now, in one ink: a hollow stroke from star
to star that swells into a round cell at each star. The magnitude is
the size of the cell and the class a mark inside it; nothing is told
by colour any more. The mapping is still v1: same stars, same edges,
same name. The terminal keeps its colours.

A gate is a second, shorter way to tell a key: six signs in order,
drawn from a catalogue of 64 that every implementation shares. The
command line prints them; the website shows them under the sky, with
its own sky as the origin. Under the gate, a chart places the key in
space: the 64 signs on a sphere, the six of the gate joined two by
two, and the key at the point nearest the three lines.

The website also lets a sky be taken away as SVG or as a square PNG,
gives a result its stardate, can play a piece of music when started
with -music, keeps its files in the browser for a year, sends text
gzipped, and asks for https.
-----BEGIN PGP SIGNATURE-----

iJEEABYKADkWIQRUIAmJF/MMEaf51e/ZPwUpucmaOAUCawinEBsUgAAAAAAEAA5t
YW51MiwyLjUrMS4xMiwwLDMACgkQ2T8FKbnJmjj0dgD/TgdZ1+340osPVQCfXg1O
9Ah7xiQPHvDAz8kuzhX+bucA/0Skloeg8b6GfEjwSf4GjD9jD5vz/qvUiW9tzrod
DP8O
=w1XV
-----END PGP SIGNATURE-----

## v0.3.0 — 2026-11-20

The announcement, kept true and said to be signed.

stellar announce keeps the DANE records of a few hosts equal to the keys
those hosts serve: it reads each key from a verified TLS handshake and
replaces the host's TLSA 3 1 1 record through Gandi LiveDNS when a
renewal changed it. A host it cannot read keeps its record; nothing is
deleted. -dry-run prints the records and needs no token.

stellar -host and the website now say whether the announcement is
signed: the TLSA question asks the resolver for authenticated data, and
the answer says when that resolver checked the zone's DNSSEC signatures.
Stellar still does not validate DNSSEC itself.

The mapping is still version 1.
-----BEGIN PGP SIGNATURE-----

iJEEABYKADkWIQRUIAmJF/MMEaf51e/ZPwUpucmaOAUCav//9BsUgAAAAAAEAA5t
YW51MiwyLjUrMS4xMiwwLDMACgkQ2T8FKbnJmjghYwEAkbvl/0n9GwwGzm1JbLkR
Pj6bms9IAgAqnGQIxTdsns4A/1kBBsSMB/SmkafU7kQFZA79tTZVLNjgQKnM1kQu
M7oJ
=Y/04
-----END PGP SIGNATURE-----

## v0.2.0 — 2026-11-20

What a site shows, what its owner announces.

For a TLS server, stellar -host now says what the handshake and the DNS
show beside the sky: the certificate's key type, expiry and issuer, the
TLS version and the key exchange negotiated, post-quantum hybrid or
not, and whether the owner announces this key in a DANE record.

stellar -o file.png writes the sky as a picture, drawn without any
dependency.

The website of stellar.mesa.black, the same binary, speaks English,
French and Spanish, illustrates its links with a picture of the sky,
shows its own sky, and can look a name up in the public certificate
logs. The module needs Go 1.27. The mapping is still version 1.
-----BEGIN PGP SIGNATURE-----

iJEEABYKADkWIQRUIAmJF/MMEaf51e/ZPwUpucmaOAUCav/UKBsUgAAAAAAEAA5t
YW51MiwyLjUrMS4xMiwwLDMACgkQ2T8FKbnJmjglcAD9G4nx4D+CEUb7o0G84i/C
qV4Asdpa/0HtCipJe4O1XzkA/iFpe0ND6lHZ31uE7v9k/Wuu2EurcHCFTDV/4sZK
XDQF
=rLgZ
-----END PGP SIGNATURE-----

## v0.1.0 — 2026-11-20

A key, drawn.

The first release of the command line: one static binary per platform.

stellar draws a public key as a constellation with an invented name, in
the terminal or as SVG: an OpenSSH key, a PEM key or certificate, a
SHA256: fingerprint, or the certificate a TLS server presents. For a
server it also says what the handshake shows, the post-quantum key
exchange included. stellar serve is the website of stellar.mesa.black.

The mapping from key to sky is version 1, specified in docs/spec.md (in
the README at the time) and pinned by testdata/v1.json.
-----BEGIN PGP SIGNATURE-----

iJEEABYKADkWIQRUIAmJF/MMEaf51e/ZPwUpucmaOAUCav/MkRsUgAAAAAAEAA5t
YW51MiwyLjUrMS4xMiwwLDMACgkQ2T8FKbnJmjgCzQD/bD3mZ9Ph6ey7IuIXPcw9
jCwFzuq6T/oRtHLrY7WMwWEA/1XNZ9UfBn1kGFCprv4WAuPfW6q5ZCZMHd4yRtFh
IdwJ
=NZh3
-----END PGP SIGNATURE-----
