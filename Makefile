BINARY=skillhub
# Default CLI version; override with `make VERSION=1.2.3` or `VERSION=1.2.3 make`.
VERSION ?= 0.0.1
GOPATH=$(shell go env GOPATH)
LDFLAGS=-ldflags="-s -w -X main.Version=${VERSION}"

.PHONY: build build-all clean test completions install

build:
	go build ${LDFLAGS} -o ${BINARY} .

test:
	go test ./... -v -count=1

install: build
	cp ${BINARY} ${GOPATH}/bin/${BINARY}

uninstall:
	rm -f ${GOPATH}/bin/${BINARY}

clean:
	rm -f ${BINARY} 
