BINARY=skillhub
# Default CLI version; override with `make VERSION=1.2.3` or `VERSION=1.2.3 make`.
VERSION ?= 0.0.2
GOPATH=$(shell go env GOPATH)
LDFLAGS=-ldflags="-s -w -X main.Version=${VERSION}"

.PHONY: build build-all clean test completions install

build:
	go build ${LDFLAGS} -o ${BINARY} .

# Cross-compile for all supported platforms
build-all:
	@mkdir -p dist
	@for pair in \
		"linux:amd64" "linux:arm64" \
		"darwin:amd64" "darwin:arm64" \
		"windows:amd64" "windows:arm64"; do \
		GOOS=$${pair%%:*} GOARCH=$${pair##*:} ; \
		EXT=""; [ "$$GOOS" = "windows" ] && EXT=".exe"; \
		OUT="dist/${BINARY}-$${GOOS}-$${GOARCH}$${EXT}"; \
		echo "Building $$OUT ..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build ${LDFLAGS} -o "$$OUT" .; \
		sha256sum "$$OUT" > "$$OUT.sha256"; \
	done
	@echo "All binaries built in dist/"

test:
	go test ./... -v -count=1

install: build
	cp ${BINARY} ${GOPATH}/bin/${BINARY}

uninstall:
	rm -f ${GOPATH}/bin/${BINARY}

clean:
	rm -f ${BINARY}
	rm -rf dist/
