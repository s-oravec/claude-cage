.PHONY: build test install clean

VERSION := 0.1.0
LDFLAGS := -ldflags "-X github.com/stiivo/cage/internal/cmd.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o cage ./cmd/cage

test:
	go test ./... -v

install: build
	install -m 755 cage ~/.local/bin/cage

clean:
	rm -f cage
