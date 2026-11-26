# Contributing

Stellar is small on purpose. A change is welcome when it keeps these four
things true.

## The rules

**The mapping does not move.** Version 1 turns a digest into a sky, a name
and a gate, and `testdata/v1.json` and `testdata/gate-v1.json` pin it:
another implementation must draw the same thing, and Sablier's already does.
A change that makes `TestGolden` or `TestGateVectors` fail is a new version
of the mapping, not a fix. How a sky is rendered may change; what it holds
may not.

**No dependency.** The library, the command line and the website use the
standard library of Go and nothing else: `go.mod` is the whole manifest. The
website loads nothing from another origin and carries no inline script or
style.

**Say what it does not prove.** A sky, a name, a gate and a chart are aids
to notice a change, and each carries far fewer bits than the fingerprint.
Text that shows one says so, in the three languages of the site.

**The site lives in little memory.** It runs within 64 MiB. Anything that
allocates per request (a picture, a compressor) is measured before it is
merged, and bounded.

## Working on it

```sh
make test     # gofmt, go vet and the tests
make serve    # the website on http://localhost:8099, from the image
```

Without Go on the machine, `make test` and `make serve` run in a container.
The tests need no network.

- Code, comments, commit messages and documentation are in English.
- A commit says what changed and why, in sentences; no prefix is required.
- The website's sentences live in `web/i18n.go`, in English, French and
  Spanish; `TestLanguagesAgree` refuses a sentence missing from one of them.

## Releases

The website follows `main`: every push is built, tested and deployed. The
command line is released by a tag, `vX.Y.Z`, whose message becomes the text
of the release; the same text goes at the top of `CHANGELOG.md`.

## Security

See [SECURITY.md](SECURITY.md): a flaw is reported in private.
