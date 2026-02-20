BINARY   := senate
CMD_DIR  := ./cmd/senate
BUILD    := go build
GOFLAGS  := -ldflags="-s -w"
PREFIX   ?= /usr/local/bin

.PHONY: build test test-race cover lint install clean

build:
	$(BUILD) $(GOFLAGS) -o $(BINARY) $(CMD_DIR)

test:
	go test ./... -count=1

test-race:
	go test ./... -count=1 -race

cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@rm -f coverage.out

lint:
	go vet ./...

install: build
	install -m 755 $(BINARY) $(PREFIX)/$(BINARY)

clean:
	rm -f $(BINARY) coverage.out
