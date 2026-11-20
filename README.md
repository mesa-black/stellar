# Stellar

Stellar draws a public key as a constellation, so that people can compare two keys by looking at a small sky instead of reading 43 characters of base64.

![The constellation "Subeve Zidu"](docs/example.svg)

It is a visual fingerprint, like OpenSSH randomart. It is **not** a cipher and it is not a new cryptosystem: all the security comes from the SHA-256 digest of the key, and Stellar only changes how that digest is shown. The project is unrelated to the Stellar payment network.

Status: prototype. The mapping is versioned (`v1`) but may still change.

## Build

Stellar needs Go 1.24 or later and has no dependency outside the standard library.

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
└─[ Subeve Zidu ]────────────────────────┘
Name:        Subeve Zidu
Fingerprint: SHA256:kqckL3tw6x8R1HqM3pohRjFi/6wO3yM2MXYGcV0/2qc
Key:         test@stellar (ssh-ed25519)
```

| Flag | Effect |
| --- | --- |
| `-o file.svg` | Write an SVG instead of drawing in the terminal (`-o -` for standard output). |
| `-size 640` | Width and height of the SVG, in pixels. |
| `-host name[:port]` | Fetch the key from a TLS server. The port defaults to 443. |
| `-insecure` | With `-host`, skip certificate verification. |

Colours are disabled when the output is not a terminal or when `NO_COLOR` is set.

## What is hashed

| Input | Digest |
| --- | --- |
| OpenSSH public key | SHA-256 of the key blob. This is the fingerprint `ssh-keygen -l` prints. |
| PEM public key, PEM certificate, TLS host | SHA-256 of the DER `SubjectPublicKeyInfo`. |
| `SHA256:` fingerprint | The fingerprint itself. |

A certificate and the public key it certifies give the same constellation. A renewed certificate keeps its constellation for as long as it keeps its key.

Stellar refuses private keys.

## Limits

- **The fingerprint is the authority.** A constellation carries fewer bits to the eye than the digest holds. Someone who can generate many keys can search for one whose constellation resembles yours. Use the drawing to notice a change quickly; compare the full fingerprint when it matters.
- **The name alone is weak.** It holds about 31.6 bits, and a matching name can be found by brute force in minutes. Never accept a key on its name only.
- **The terminal view is a subset.** It shows the main stars and the edges, not the background stars.
- **`-insecure` shows what the network presents.** Without verification, the key may come from anyone on the path.
- **No human study has been run.** How reliably people tell two constellations apart is not measured yet.

## Algorithm (v1)

The mapping from digest to constellation is deterministic. All integers below are drawn, in the order given, from one stream.

**Stream.** SHAKE256 absorbs the 11 bytes `stellar/v1\0` followed by the 32-byte digest. To draw a uniform integer in `[0, n)`, read 2 bytes as a big-endian integer `v`; if `v >= 65536 - (65536 mod n)`, discard it and read again; otherwise return `v mod n`.

1. **Name.** A word of three syllables, then a word of two. Each syllable is a consonant drawn from `bdfghjklmnprstvz` followed by a vowel drawn from `aeiou`. The first letter of each word is capitalised.
2. **Star count.** `n = 7 + draw(6)`, so 7 to 12 main stars.
3. **Main stars.** Until `n` stars are placed: draw `x = draw(64)` and `y = draw(64)`. If the point is closer than 9 grid units to a star already placed (squared distance below 81), discard it. Otherwise draw `magnitude = draw(3)` and `class = draw(5)` and place the star. Magnitude 0 is the brightest; classes run from blue (0) through white, yellow and orange to red (4).
4. **Tree.** For each star `i` from 1 to `n-1`, sort the stars placed before it by increasing squared distance to `i`, ties broken by lower index. Star 1 joins star 0. Every later star joins candidate number `draw(2)` of that list.
5. **Loops.** Draw `k = draw(3)`. Repeat `k` times: draw `a = draw(n)` and join `a` to the nearest star it is not already joined to (same ordering as above). If there is none, do nothing.
6. **Background.** Draw `m = 40 + draw(24)`. For each of the `m` background stars draw `x = draw(256)`, `y = draw(256)` and `brightness = draw(3)`.

Rendering is not normative, but a `v1` renderer must show every main star with its position, magnitude and class, and every edge.

`TestGolden` pins this mapping. A change that alters the output for an existing digest must bump the version, in the domain separator as well as in `Version`.

## Library

```go
key, err := stellar.Parse(data) // or ParseSSH, ParsePEM, ParseFingerprint, FromHost
c := stellar.New(key.Digest)

fmt.Print(c.Text(true))               // terminal
svg := c.SVG(640, key.Fingerprint())  // SVG document
```
