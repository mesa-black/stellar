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
FROM docker.io/library/golang:1.27-alpine AS build
WORKDIR /src
# No dependency outside the standard library: go.mod is the whole manifest.
COPY go.mod ./
COPY . .
# The image is not built if the checks fail.
RUN test -z "$(gofmt -l .)" && go vet ./... && go test ./...
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/stellar ./cmd/stellar

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
