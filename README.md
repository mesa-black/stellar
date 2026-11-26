# Stellar

Stellar draws a public key as a constellation, so that people can compare two keys by looking at a small sky instead of reading 43 characters of base64.

![The constellation "Subevia Zidensis"](docs/example.svg)

It is a visual fingerprint, like OpenSSH randomart. It is **not** a cipher and it is not a new cryptosystem: all the security comes from the SHA-256 digest of the key, and Stellar only changes how that digest is shown. The project is unrelated to the Stellar payment network.

Status: prototype. The mapping is versioned (`v1`) but may still change.

## Install

The command line is one static binary, with no dependency.

```sh
# with Go 1.27 or later
go install github.com/mesa-black/stellar/cmd/stellar@latest

# or a binary from the releases page, here for a Mac with Apple silicon
curl -LO https://github.com/mesa-black/stellar/releases/latest/download/stellar-darwin-arm64
chmod +x stellar-darwin-arm64 && mv stellar-darwin-arm64 /usr/local/bin/stellar

# or without installing anything
podman run --rm ghcr.io/mesa-black/stellar:<commit> -host example.com
```

Binaries exist for `linux` and `darwin`, on `amd64` and `arm64`; `SHA256SUMS` beside them lists their checksums. `stellar version` says which build is running.

## Build

Stellar needs Go 1.27 or later and has no dependency outside the standard library. Without Go, `make image` builds everything in a container, with Podman or Docker (see [Container](#container)).

```sh
go build -o bin/stellar ./cmd/stellar
go test ./...
```

## Usage

```sh
stellar ~/.ssh/id_ed25519.pub        # an OpenSSH public key
stellar cert.pem                     # a PEM certificate or public key
stellar -host example.com            # the certificate a TLS server presents
stellar SHA256:kqckL3tw6x8R1Hq...    # a fingerprint someone sent you
cat key.pub | stellar -              # standard input
```

By default the constellation is drawn in the terminal:

```
┌─[ stellar v1 ]─────────────────────────┐
│             *⣶⠤⣄⡀                      │
│        ✦     ⠈⠳⣄⠉⠓⠲⢤⣀                  │
│        ⠹⣄      ⠈⠳⣄  ⠈⠉⠓⠦⣄⣀             │
│         ⠘⢦ ✦     ⠈⢧⡀     ⠈⠙⠲⠤⣄⡀        │
│          ⠈⠳⣷⡀      ⠙⢦⡀        ✦        │
│            ⠙⣷⡀       ⠙⢦⡀      ⠸⡄       │
│             ⠈•⠒⠒⠒⠒⠒⠒⠒⠒⢒⡿*⣄     ⣇       │
│                      ⣠⠞⠁ ⠈⢳⡀   ⢸       │
│                    ⢠⡾⠃     ⠙⢦⡀ ⠈⡇      │
│                   *⠋         ⠙⢦⡀⢹      │
│                 ⢠⠞⠙⠲⣄⡀         ⠳⣜⡆     │
│               ⢀⡴⠋    ⠙⠲⣄⡀       ⠈✦     │
│              ⣠⠞         ⠙⠲⣄⡀     ⢹⡄    │
│            ⢀⡞⠁             ⠙⠲⣄⡀  ⠈⣇    │
│           ⡴⠋                  ⠙⠲⣄⡀⢹⡀   │
│          *⠁                      ⠙⠺•   │
└─[ Subevia Zidensis ]───────────────────┘
Name:        Subevia Zidensis
Fingerprint: SHA256:kqckL3tw6x8R1HqM3pohRjFi/6wO3yM2MXYGcV0/2qc
Key:         test@stellar (ssh-ed25519)
```

| Flag | Effect |
| --- | --- |
| `-o file.svg` | Write an SVG instead of drawing in the terminal (`-o -` for standard output). A name ending in `.png` writes a PNG, without the name and fingerprint. |
| `-size 640` | Width and height of the picture, in pixels. |
| `-host name[:port]` | Fetch the key from a TLS server. The port defaults to 443. |
| `-insecure` | With `-host`, skip certificate verification. |

Colours are disabled when the output is not a terminal or when `NO_COLOR` is set.

With `-host`, three more lines say what the certificate, the handshake and the DNS show:

```
Certificate: ECDSA P-256, until 2027-02-18, Let's Encrypt YE2
Connection:  TLS 1.3, X25519MLKEM768 (post-quantum hybrid)
Announced:   no, the domain publishes no DANE record
```

The key exchange is the one negotiated with Stellar itself, which offers the hybrid post-quantum group first: a server that supports it is seen to. "Post-quantum" there is about the secrecy of the session. The key that is drawn is a classical one, as in every certificate issued today.

## Website

`stellar serve` runs a small website: type a host name, see the sky of the key its certificate carries.

```sh
make serve                # builds the image in a container, then http://localhost:8099
stellar serve -addr :8080 # or straight from the binary
```

| Route | Purpose |
| --- | --- |
| `/` and `/?host=example.com` | The page. A whole URL may be pasted; only its host name is kept. A `SHA256:` fingerprint in the same field draws that key directly, with nothing fetched. |
| `/sky/example.com.svg` | The constellation as an image, to embed elsewhere. `?size=` sets its side, from 64 to 2048 pixels. |
| `/sky/example.com.png` | The same as a PNG, 1024 pixels square, without the name and fingerprint. |
| `/card/example.com.png` | The same sky as a 1200 × 630 picture, for the preview shown under a shared link. Pages name it in their `og:image`. |
| `POST /key` | The constellation of a pasted public key, certificate or fingerprint. |
| `/healthz` | Liveness probe. |

**The site's own sky.** Started with its public address (`-url`), the site shows on its empty page the sky of its own key, fetched like any other and refreshed in the background, so a visitor never waits for it.

**What the owner announces.** A domain can say in its DNS which key to expect from it: a TLSA record (DANE, RFC 6698) at `_443._tcp.<host>`. The form everyone is advised to publish, `3 1 1`, carries the SHA-256 of the key, which is exactly the digest a sky is drawn from. So Stellar reads that record as a sky: under a site's own, the page says whether the owner announces this key, another one (named), or none, in which case it shows the record to publish. `stellar -host` prints the same on its `Announced:` line.

Stellar does not validate DNSSEC itself: it asks the resolver the server uses whether it did (the answer's AD flag) and says so. "Signed" means that resolver checked the zone's signatures; without it, the record is what the DNS said and no more. Either way it is a second road, independent of the certificate authorities, and as trustworthy as that resolver. Go's resolver does not ask for TLSA records, so the question is written out in `dane.go`; it goes to the machine's own resolver and to no address a visitor chose.

**Keeping the announcement true.** A certificate that renews with a new key makes yesterday's record wrong. `stellar announce` closes that gap: it reads the key each host serves, as a visitor would, and makes the host's `TLSA 3 1 1` record say the same.

```sh
stellar announce -zone example.org -dry-run example.org www.example.org   # print the records, publish nothing
GANDI_TOKEN=… stellar announce -zone example.org example.org www.example.org
```

It is meant to run every few minutes. What it does and does not do:

- The key comes from a TLS handshake whose certificate chain verifies. No private key is read, and a host that does not answer, or answers with a certificate the machine does not trust, keeps the record it had. Nothing is ever deleted.
- Between a renewal and the next run, plus the record's lifetime in caches (`-ttl`, 300 seconds by default), the DNS announces the previous key. A client that enforces DANE would refuse the site for those minutes; browsers do not enforce it.
- The record set at `_443._tcp.<host>` belongs to the command: anything else published there is replaced.
- The zone must be signed with DNSSEC for the record to be worth more than the DNS it travels in.
- One provider is supported: Gandi LiveDNS, with a personal access token limited to the zone, read from `GANDI_TOKEN` and sent to `api.gandi.net` only.

**In the public logs.** Started with `-logs`, the site offers, under a host's sky, to look the name up in the public Certificate Transparency logs and to draw the keys that hold a valid certificate for it today. More than one is ordinary around a renewal; one nobody recognises is a certificate somebody obtained for the name; and the page says when the key the site showed is not among them. The logs are read through Cert Spotter (SSLMate), whose answers carry the SHA-256 of each key. That has two consequences, both said on the page: the name is sent to that service, so nothing is asked until the visitor asks, and it lists unexpired certificates only, so this is the present and not a history. Its free interface allows a hundred questions an hour; answers are kept four hours.

**Languages.** The site speaks English, French and Spanish. It answers in the language the browser asks for (`Accept-Language`), and a visitor can choose another at the foot of the page; that choice travels in the address (`?lang=fr`), so a link keeps the language it was copied in. English governs: `web/i18n.go` holds the three catalogues, and a test refuses one whose sentences or placeholders differ from the English.

**What the handshake says.** Under the sky of a site, the page gives the certificate's key type, its expiry and its issuer, the TLS version, the key exchange negotiated, and whether that exchange is a post-quantum hybrid: whether what travels to the site today could be recorded now and read by a quantum computer later.

**Remembering a sky.** The page stores, in the visitor's browser only (`localStorage`), the fingerprint each host showed last. On a later visit it says whether the sky is the same or has changed. A change is not proof of an attack: many sites generate a new key at every certificate renewal, so their sky changes every few weeks. Sites that keep their key across renewals keep their sky.

**Taking a sky away.** Under a host's sky, two links download it: `/sky/example.com.svg`, with the name and the fingerprint written in it, and `/sky/example.com.png`, a 1024-pixel square without text.

**Music.** Off by default. `stellar serve -music https://… -music-credit "…"` plays the file at that address on the pages, in a loop, from the visitor's first click, with a button pinned to the corner to cut it; the choice is remembered in the browser. The file is not served by Stellar: visitors fetch it from that address, whose server sees theirs, and only once the music starts. The content security policy opens to that host and the hosts under it, for media alone. Whether a piece may be played on your site is yours to know.

**Sharing a sky.** Once a sky is drawn, a "Send on Threema" link opens Threema with a message naming the host, its sky and its fingerprint, to edit before sending. It is a plain link: nothing loads from Threema until someone clicks. Start the site with its public address (`stellar serve -url https://stellar.example`, or `make serve URL=…`) and the message ends with a link back to the sky; without it, the message carries no link.

**What the server agrees to connect to.** The site connects to hosts named by strangers, so it is restricted:

- Only DNS names are accepted: no IP address, no port, no single-label name. The port is always 443.
- The address actually dialled is checked after name resolution and must be public. Loopback, private, link-local, carrier-grade NAT, NAT64 and 6to4 ranges are refused, whatever the DNS answers.
- The certificate chain is verified. A key that cannot be verified is not drawn.
- At most 16 lookups run at once, each within 8 seconds. Results are cached for 5 minutes.
- Errors shown to visitors never echo the underlying network error.

There is no per-visitor rate limit: put the site behind a reverse proxy that has one before exposing it to the internet.

The pages load nothing from other origins and carry no inline script or style, under a `default-src 'none'` content security policy. The typefaces (Bricolage Grotesque and IBM Plex Mono, both under the SIL Open Font License) are served by the site itself; see `web/static/fonts/NOTICE.txt`.

### Container

```sh
make test    # format check, vet and tests, inside the build stage
make image   # the image: a static binary and the root certificates, about 9 MB
make serve   # run it, read-only, without capabilities, on 127.0.0.1:8099 (PORT=… to change)
make sky HOST=example.com
```

The image is built `FROM scratch` and runs as an unprivileged user. Go is only needed inside the build stage, so a workstation with a container engine needs nothing else.

**Podman or Docker.** Both work, with the same `Dockerfile` and the same commands. The Makefile uses Podman by default; pass `ENGINE=docker` to use Docker:

```sh
make serve ENGINE=docker
```

Without `make`, the two commands are:

```sh
podman build -t stellar .                          # or: docker build -t stellar .
podman run --rm -p 127.0.0.1:8099:8080 stellar     # or: docker run  --rm -p 127.0.0.1:8099:8080 stellar
```

### Deployment

The site runs at <https://stellar.mesa.black>. A push to `main` is a deployment: the CI runs the checks, publishes `ghcr.io/mesa-black/stellar:<commit>` for amd64 and arm64, then writes that tag and its digest into the deployment repository, which Argo CD applies to the cluster. The CI never talks to the cluster, and rolling back is reverting that commit.

## What is hashed

| Input | Digest |
| --- | --- |
| OpenSSH public key | SHA-256 of the key blob. This is the fingerprint `ssh-keygen -l` prints. |
| PEM public key, PEM certificate, TLS host | SHA-256 of the DER `SubjectPublicKeyInfo`. |
| `SHA256:` fingerprint | The fingerprint itself. |

A certificate and the public key it certifies give the same constellation. A renewed certificate keeps its constellation for as long as it keeps its key.

Stellar refuses private keys.

## Names

Every sky has a name, such as *Subevia Zidensis* or *Merebis Vovinus*. The names are **invented**: they are derived from the key, like the stars, and none is the name of a real constellation. The drawings do not match real constellations either.

They are built to sound like the Latin names astronomers use (*Corona Borealis*, *Piscis Austrinus*): a noun with a Latin ending (`-us`, `-ia`, `-ix`…) followed by an epithet (`-alis`, `-ensis`, `-inus`…). The point is a name that can be said aloud, over the phone or across a desk: "on my side your site is Merebis Vovinus; and on yours?"

Real constellation names would not do. There are 88 official ones, so unrelated keys would share a name all the time, and making a key that is called *Orion* like yours would take a few dozen tries. The invented scheme has about 8.4 billion names (33 bits). That makes an accidental match unlikely; it does not stop someone who searches for one on purpose (see [Limits](#limits)).

The names have no meaning in Latin, and no care is taken to make noun and epithet agree.

## Limits

- **The fingerprint is the authority.** A constellation carries fewer bits to the eye than the digest holds. Someone who can generate many keys can search for one whose constellation resembles yours. Use the drawing to notice a change quickly; compare the full fingerprint when it matters.
- **The name alone is weak.** It holds about 33 bits, and a matching name can be found by brute force in minutes. Never accept a key on its name only.
- **The terminal view is a subset.** It shows the main stars and the edges, not the background stars.
- **The website sees the key from where the server sits.** It shows the key the site presented to the Stellar server, not the one it presented to your browser. Someone intercepting your own connection is not visible from there. To check what your machine receives, run the command line locally: `stellar -host example.com`.
- **`-insecure` shows what the network presents.** Without verification, the key may come from anyone on the path.
- **No human study has been run.** How reliably people tell two constellations apart is not measured yet.

## Algorithm (v1)

The mapping from digest to constellation is deterministic. All integers below are drawn, in the order given, from one stream.

**Stream.** SHAKE256 absorbs the 11 bytes `stellar/v1\0` followed by the 32-byte digest. To draw a uniform integer in `[0, n)`, read 2 bytes as a big-endian integer `v`; if `v >= 65536 - (65536 mod n)`, discard it and read again; otherwise return `v mod n`.

1. **Name.** Two words. The noun is a consonant, a vowel, a consonant, a vowel, a consonant and a noun ending. The epithet is a consonant, a vowel, a consonant and an epithet ending. Consonants are drawn from `bdfghjklmnprstvz` (`draw(16)`), vowels from `aeiou` (`draw(5)`), noun endings from `a us um is ix or ax ia` (`draw(8)`), epithet endings from `alis aris ensis inus ianus osus atus icus` (`draw(8)`), each list indexed from 0. The first letter of each word is capitalised.
2. **Star count.** `n = 7 + draw(6)`, so 7 to 12 main stars.
3. **Main stars.** Until `n` stars are placed: draw `x = draw(64)` and `y = draw(64)`. If the point is closer than 9 grid units to a star already placed (squared distance below 81), discard it. Otherwise draw `magnitude = draw(3)` and `class = draw(5)` and place the star. Magnitude 0 is the brightest; classes run from blue (0) through white, yellow and orange to red (4).
4. **Tree.** For each star `i` from 1 to `n-1`, sort the stars placed before it by increasing squared distance to `i`, ties broken by lower index. Star 1 joins star 0. Every later star joins candidate number `draw(2)` of that list.
5. **Loops.** Draw `k = draw(3)`. Repeat `k` times: draw `a = draw(n)` and join `a` to the nearest star it is not already joined to (same ordering as above). If there is none, do nothing.
6. **Background.** Draw `m = 40 + draw(24)`. For each of the `m` background stars draw `x = draw(256)`, `y = draw(256)` and `brightness = draw(3)`.

Rendering is not normative, but a `v1` renderer must show every main star with its position, magnitude and class, and every edge.

The pictures draw the figure as a glyph, in one ink: a hollow stroke from star to star, and a round cell at each star. The size of the cell is the magnitude, the brighter the larger. The mark in the cell is the class: nothing (0), a point (1), a full cell (2), a bar (3), a cross (4). The terminal has no room for marks and tells the class by colour.

`TestGolden` pins this mapping, and `testdata/v1.json` lists the constellations of 32 digests in full: another implementation of v1 must reproduce every entry. A change that alters the output for an existing digest must bump the version, in the domain separator as well as in `Version`.

### Gate (v1)

A gate is a second, shorter way to tell a key: six signs in order, the way a gate is dialled. The signs come from a catalogue every implementation shares, so they do not look like the sky of the key: they lead to it, as coordinates do. Two keys with the same six signs in the same order are the same key as far as an aid can tell: six distinct signs out of 64 carry about 36 bits, close to what the name carries and far less than the fingerprint.

1. **Catalogue.** 64 signs. Sign `k`, from 0 to 63, is the v1 constellation of `SHA-256("stellar/sign/v1\0" ‖ k)`, with `k` as one byte. Only its figure and its name are used.
2. **Signs of a key.** A stream of its own: SHAKE256 absorbs the 16 bytes `stellar/gate/v1\0` followed by the 32-byte digest, and integers are drawn from it as above. Draw `draw(64)` until six distinct signs are held, discarding a sign already taken. Their order is the order they were drawn in.
3. **Origin.** Whoever shows a gate may add a seventh sign: the sky of its own key, drawn whole. It tells where the key was read from, and nothing about the key.

The website shows the gate under the sky, with its own sky as the origin once it knows its public address (`-url`). The command line prints the six signs by the first word of their names. `TestGateVectors` pins the catalogue and the gates, and `testdata/gate-v1.json` lists them for the digests of `testdata/v1.json`.

### Chart

Under the gate, the website draws where the key stands in space. This is a picture of the gate and nothing more: it carries no bit the six signs do not carry, and it is not part of v1.

- **The signs.** The 64 signs of the catalogue stand at fixed places on a sphere of radius one, along a spiral from one pole to the other: sign `k` is at height `y = 1 − 2(k + ½)/64`, turned by `k` golden angles (`π(3 − √5)` each).
- **The arrival.** The six signs of a gate, taken two by two in their order (1–2, 3–4, 5–6), draw three lines through the sphere, each from one sign to the other. Three lines in space seldom meet: the place of the key is the point nearest the three of them (least sum of squared distances). The chart ties it by a dotted stroke to the nearest point of each line: it is the middle of those three points. `stellar.SignPlace` and `stellar.Place` compute both.
- **The origin.** Once the site knows its own key, its own place is found the same way, then pushed out of the sphere in the same direction: the origin is not one of the places the signs lead to. A dashed line joins it to the arrival.

The chart is drawn by the server and stands without a script. With one, it turns slowly, and under the mouse or the finger; it stays still for a visitor who asked their system for less motion.

**Stardate.** Among the facts of a result, the page gives the moment it was drawn the way a ship's log would: the year, then the share of that year already gone, to two decimals (`2026.90` is nine tenths into 2026). It is a wink, and nothing is computed from it.

## Library

```go
key, err := stellar.Parse(data) // or ParseSSH, ParsePEM, ParseFingerprint, FromHost
c := stellar.New(key.Digest)

fmt.Print(c.Text(true))               // terminal
svg := c.SVG(640, key.Fingerprint())  // SVG document
```

## License

Stellar is released under the [MIT License](LICENSE).

The typefaces in `web/static/fonts/` are not part of Stellar and keep their own licence, the SIL Open Font License: see `web/static/fonts/NOTICE.txt`.
