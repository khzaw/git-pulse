APP := git-pulse

.PHONY: build test

build:
	go build ./cmd/$(APP)

test:
	go test ./...
