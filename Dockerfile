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
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27-alpine AS build
WORKDIR /src
# No dependency outside the standard library: go.mod is the whole manifest.
COPY go.mod ./
COPY . .
# The image is not built if the checks fail.
RUN test -z "$(gofmt -l .)" && go vet ./... && go test ./...
# Set by the builder to the platform the image is for; empty means this one.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/stellar ./cmd/stellar

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
