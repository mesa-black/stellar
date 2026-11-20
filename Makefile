.DEFAULT_GOAL := help
.PHONY: help test image serve stop sky

# Everything runs in containers: the workstation needs podman, not Go.
IMAGE ?= localhost/stellar
PORT  ?= 8099
TTY   := $(shell [ -t 0 ] && echo -t)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

test: ## Check formatting, vet and run the tests
	podman build --target build -t $(IMAGE)-build .

image: ## Build the image (runs the tests first)
	podman build -t $(IMAGE) .

serve: image ## Serve the website on http://localhost:8099 (PORT=… to change)
	podman run --rm -i $(TTY) --name stellar --read-only --cap-drop=ALL \
		--security-opt=no-new-privileges -p 127.0.0.1:$(PORT):8080 $(IMAGE)

stop: ## Stop the website started by `make serve`
	-podman stop stellar

sky: image ## Draw a key in the terminal: make sky KEY=~/.ssh/id_ed25519.pub or HOST=example.com
ifdef HOST
	@podman run --rm -i $(TTY) $(IMAGE) -host $(HOST)
else
	@test -n "$(KEY)" || { echo "usage: make sky KEY=<public key file> | HOST=<name>"; exit 2; }
	@podman run --rm -i $(TTY) $(IMAGE) - < $(KEY)
endif
