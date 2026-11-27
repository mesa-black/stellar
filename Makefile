.DEFAULT_GOAL := help
.PHONY: help test image serve stop sky extension browser

# Everything runs in containers: the workstation needs a container engine, not
# Go. Podman by default; `make serve ENGINE=docker` works the same.
ENGINE ?= podman
IMAGE  ?= localhost/stellar
PORT   ?= 8099
TTY    := $(shell [ -t 0 ] && echo -t)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

test: ## Check formatting, vet and run the tests
	$(ENGINE) build --target build -t $(IMAGE)-build .

# In the image that compiles Stellar, with the Firefox of its distribution:
# nothing to install on the machine, and the same browser for everybody.
browser: ## Drive the site in a real Firefox, without a window, and check what it shows
	@$(ENGINE) run --rm -v "$(CURDIR):/src:ro" -w /src \
		-e GOFLAGS=-buildvcs=false -e GOCACHE=/tmp/gocache -e HOME=/tmp -e STELLAR_BROWSER_REQUIRED=1 \
		$$(sed -n 's/^FROM --platform=[^ ]* \([^ ]*\) AS build$$/\1/p' Dockerfile) \
		sh -c 'apk add --no-cache firefox >/dev/null && go test -count=1 -run TestBrowser -v ./web'

extension: ## Package the Firefox extension into bin/stellar-extension.zip
	@mkdir -p bin && rm -f bin/stellar-extension.zip
	@cd extension && zip -q -r -X ../bin/stellar-extension.zip . -x test.js -x README.md -x LISTING.md
	@printf '  %s\n' "bin/stellar-extension.zip"

image: ## Build the image (runs the tests first)
	$(ENGINE) build -t $(IMAGE) .

serve: image ## Serve the website on http://localhost:8099 (PORT=…, URL=https://… for share links)
	$(ENGINE) run --rm -i $(TTY) --name stellar --read-only --cap-drop=ALL \
		--security-opt=no-new-privileges -p 127.0.0.1:$(PORT):8080 $(IMAGE) \
		serve -addr :8080 $(if $(URL),-url $(URL))

stop: ## Stop the website started by `make serve`
	-$(ENGINE) stop stellar

sky: image ## Draw a key in the terminal: make sky KEY=~/.ssh/id_ed25519.pub or HOST=example.com
ifdef HOST
	@$(ENGINE) run --rm -i $(TTY) $(IMAGE) -host $(HOST)
else
	@test -n "$(KEY)" || { echo "usage: make sky KEY=<public key file> | HOST=<name>"; exit 2; }
	@$(ENGINE) run --rm -i $(TTY) $(IMAGE) - < $(KEY)
endif
