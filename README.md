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
| `-with key` | Compare with another key: a file, a `SHA256:` fingerprint, or a TLS server written `host:name[:port]`. Prints the other key's sky on a `With:` line and, on `Apart:`, how far the two stand from each other in the sphere of the signs, in radii (0 for the same key). |
| `-chart file.svg` | Write the chart of the key to a file: its gate in the sphere of the signs and the place it leads to, with the key given to `-with` beside it. |

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

**On the wire.** The files of the site are linked under a version made from their contents (`/static/<version>/…`), so a browser keeps them for a year and never asks again; when one changes, its address does. Pages, styles, scripts and SVG drawings travel gzipped, about four times smaller. No more than four answers are compressed at once, because a compressor holds a megabyte and the site is meant to live in very little memory: a fifth goes as it is. The server also sends `Permissions-Policy` and `Cross-Origin-Opener-Policy`, and, started with an `https` address (`-url`), `Strict-Transport-Security` for a year.

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

## Limits

- **The fingerprint is the authority.** A constellation carries fewer bits to the eye than the digest holds. Someone who can generate many keys can search for one whose constellation resembles yours. Use the drawing to notice a change quickly; compare the full fingerprint when it matters.
- **The name alone is weak.** It holds about 33 bits, and a matching name can be found by brute force in minutes. Never accept a key on its name only.
- **The terminal view is a subset.** It shows the main stars and the edges, not the background stars.
- **The website sees the key from where the server sits.** It shows the key the site presented to the Stellar server, not the one it presented to your browser. Someone intercepting your own connection is not visible from there. To check what your machine receives, run the command line locally: `stellar -host example.com`. In Firefox, the extension in [`extension/`](extension/) does it for the page in front of you, and asks the website to compare.
- **`-insecure` shows what the network presents.** Without verification, the key may come from anyone on the path.
- **No human study has been run.** How reliably people tell two constellations apart is not measured yet.

## Specification

The mapping from a key to its sky, its name and its gate is version 1, and it does not move: [docs/spec.md](docs/spec.md) says what is hashed, how the name is made, and gives the algorithm step by step, with the gate and the chart. `testdata/v1.json` and `testdata/gate-v1.json` hold the reference skies and gates another implementation must reproduce.

## Library

```go
key, err := stellar.Parse(data) // or ParseSSH, ParsePEM, ParseFingerprint, FromHost
c := stellar.New(key.Digest)

fmt.Print(c.Text(true))               // terminal
svg := c.SVG(640, key.Fingerprint())  // SVG document
```

## Contributing

The rules that keep Stellar what it is are in [CONTRIBUTING.md](CONTRIBUTING.md), what each release brought in [CHANGELOG.md](CHANGELOG.md), and how to report a flaw, in private, in [SECURITY.md](SECURITY.md).

## License

Stellar is released under the [MIT License](LICENSE).

The typefaces in `web/static/fonts/` are not part of Stellar and keep their own licence, the SIL Open Font License: see `web/static/fonts/NOTICE.txt`.
