# syntax=docker/dockerfile:1
#
# Stellar as a container image: one static binary, the website on port 8080.
#
#   podman build -t stellar .
#   podman run --rm -p 127.0.0.1:8080:8080 stellar
#
# Docker builds and runs it the same way: replace `podman` with `docker`.
#
# Two stages. The build stage is also the project's Go toolchain: the Makefile
# runs the tests in it (`podman build --target build`), so a workstation needs
# a container engine and nothing else.

# ------------------------------------------------------------------ build
# On the BUILD machine's architecture ($BUILDPLATFORM), whatever the image is
# for: Go cross-compiles, so an image for amd64 and arm64 is two compilations
# on one machine and nothing runs under emulation. The checks run once, on the
# machine that builds.
# Pinned by digest: a tag is a pointer somebody else moves, and what compiles
# the binary should be the bytes that were checked. Update it on purpose.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
# No dependency outside the standard library: go.mod is the whole manifest.
COPY go.mod ./
COPY . .
# The image is not built if the checks fail.
RUN test -z "$(gofmt -l .)" && go vet ./... && go test ./...
# Set by the builder to the platform the image is for; empty means this one.
ARG TARGETOS
ARG TARGETARCH
# What `stellar version` prints: the commit for an image, the tag for a release.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/stellar ./cmd/stellar

# ------------------------------------------------------------------ dist
# The command line, for people who do not want a container: one binary per
# platform and their checksums, taken out of the build with
#   docker build --target dist --build-arg VERSION=v1.2.3 --output dist .
# Not part of the image: the stage after this one is the default.
FROM build AS dist-build
ARG VERSION=dev
RUN set -e; for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
      CGO_ENABLED=0 GOOS=${target%/*} GOARCH=${target#*/} \
      go build -trimpath -ldflags="-s -w -X main.version=$VERSION" \
        -o /dist/stellar-${target%/*}-${target#*/} ./cmd/stellar; \
    done; cd /dist && sha256sum stellar-* > SHA256SUMS

FROM scratch AS dist
COPY --from=dist-build /dist /

# ------------------------------------------------------------------ run
# Nothing but the binary and the root certificates it verifies sites against:
# no shell, no package manager, no user database.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/stellar /stellar
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/stellar"]
CMD ["serve", "-addr", ":8080"]
