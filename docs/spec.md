# Stellar, the specification

What turns a key into a sky, a name and a gate, written so that another
program can do the same and draw the same thing. Version 1 of the mapping is
pinned by `testdata/v1.json` and `testdata/gate-v1.json`: an implementation
that reproduces those files is right, and one that does not is not Stellar.
Sablier carries one, in PHP.

What is normative is the mapping: the stars, the edges, the field, the name
and the gate of a digest. How they are drawn is not, within the limit the
algorithm states. The chart is a picture of the gate and is not part of v1.

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

The website shows the gate under the sky, with its own sky as the origin once it knows its public address (`-url`). Under it, a key can be checked: a visitor who was given the key of the host by another road, as a file, a line of text or a fingerprint, pastes it, and the site says whether it is the key the host serves. The answer comes from the two digests, compared whole. The six signs of the key given are shown, each lit green where the host has the same sign at that place, and the page's script spins them like a gate being dialled before it shows the verdict; they are the picture of the answer, never the answer, and when six signs agree on two different keys the page says so. A private key is refused before it is read, by the page before it is sent and by the server if it was. When the two keys differ and the site reads the public logs (`-logs`), it offers to look for the key given in them: a large site serves several keys, and one found there for this name is one of its own, while the certificate of somebody in the way is never logged. Asking sends the name of the host to the log service, so it is done only on request. The command line prints the six signs by the first word of their names. `TestGateVectors` pins the catalogue and the gates, and `testdata/gate-v1.json` lists them for the digests of `testdata/v1.json`.

### Chart

Under the gate, the website draws where the key stands in space. This is a picture of the gate and nothing more: it carries no bit the six signs do not carry, and it is not part of v1.

- **The signs.** The 64 signs of the catalogue stand at fixed places on a sphere of radius one, along a spiral from one pole to the other: sign `k` is at height `y = 1 − 2(k + ½)/64`, turned by `k` golden angles (`π(3 − √5)` each).
- **The arrival.** The six signs of a gate, taken two by two in their order (1–2, 3–4, 5–6), draw three lines through the sphere, each from one sign to the other. Three lines in space seldom meet: the place of the key is the point nearest the three of them (least sum of squared distances). The chart ties it by a dotted stroke to the nearest point of each line: it is the middle of those three points. `stellar.SignPlace` and `stellar.Place` compute both.
- **The origin.** Once the site knows its own key, its own place is found the same way, then pushed out of the sphere in the same direction: the origin is not one of the places the signs lead to. A dashed line joins it to the arrival.

Another host can be placed on the same chart (`?host=example.com&with=example.org`): its place is drawn in green, joined to the first, and the page says how far apart the two stand, in radii of the sphere. Two hosts that serve the same key share one place. The chart is drawn by the server and stands without a script. It can be taken away as a file, `/chart/example.com.svg`, which carries its own colours, and `stellar` prints the place as `Place:`, three coordinates within the sphere. With one, it turns slowly, and under the mouse or the finger; it stays still for a visitor who asked their system for less motion.

**Stardate.** Among the facts of a result, the page gives the moment it was drawn the way a ship's log would: the year, then the share of that year already gone, to two decimals (`2026.90` is nine tenths into 2026). It is a wink, and nothing is computed from it.
