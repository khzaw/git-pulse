APP := git-pulse
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
CMD := ./cmd/$(APP)
GO_FILES := $$(find . -name '*.go' -not -path './bin/*')

.DEFAULT_GOAL := build

.PHONY: build run install fmt test test-race check clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(CMD)

run: build
	./$(BIN)

install:
	go install $(CMD)

fmt:
	gofmt -w $(GO_FILES)

test:
	go test ./...

test-race:
	go test -race ./...

check: fmt test

clean:
	rm -rf $(BIN_DIR)
