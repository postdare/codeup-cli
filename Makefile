VERSION ?= 0.1.0
DIST    := dist
MAIN    := ./cmd/codeup
LDFLAGS := -s -w -X github.com/postdare/codeup-cli/internal/cli.version=$(VERSION)

# 平台列表：mac（Intel/Apple Silicon）、Windows、Linux
PLATFORMS := darwin/arm64 darwin/amd64 windows/amd64 windows/arm64 linux/amd64

.PHONY: build install release clean test

build:
	go build -ldflags "$(LDFLAGS)" -o codeup $(MAIN)

install:
	go install -ldflags "$(LDFLAGS)" $(MAIN)

test:
	go vet ./...
	go test ./...

release: clean
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out=$(DIST)/codeup-$(VERSION)-$$os-$$arch; \
		mkdir -p $$out; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $$out/codeup$$ext $(MAIN) || exit 1; \
		cp README.md $$out/; \
		if [ "$$os" = "windows" ]; then \
			(cd $(DIST) && zip -qr codeup-$(VERSION)-$$os-$$arch.zip codeup-$(VERSION)-$$os-$$arch); \
		else \
			tar -czf $$out.tar.gz -C $(DIST) codeup-$(VERSION)-$$os-$$arch; \
		fi; \
		rm -rf $$out; \
	done
	@ls -lh $(DIST)

clean:
	rm -rf $(DIST) codeup
