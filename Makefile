PREFIX    ?= $(HOME)/.local
BINDIR    := $(PREFIX)/bin
CONFIGDIR := $(HOME)/.config/linkify

.PHONY: all build test install install-config uninstall clean

all: build

build:
	go build -o lfy ./cmd/lfy

test:
	go test ./...

install: build install-config
	@mkdir -p $(BINDIR)
	cp lfy $(BINDIR)/lfy
	@echo "lfy installed to $(BINDIR)/lfy"
	@echo ""
	@echo "Set up the URL handler and background service:"
	@echo "  lfy service install"

install-config:
	@mkdir -p $(CONFIGDIR)
	@if [ ! -f $(CONFIGDIR)/config.yaml ]; then \
		cp config.example.yaml $(CONFIGDIR)/config.yaml; \
		echo "Created $(CONFIGDIR)/config.yaml"; \
	else \
		echo "Config already exists at $(CONFIGDIR)/config.yaml (skipped)"; \
	fi

uninstall:
	-$(BINDIR)/lfy service uninstall 2>/dev/null
	rm -f $(BINDIR)/lfy
	@echo "lfy uninstalled"

clean:
	rm -f lfy
