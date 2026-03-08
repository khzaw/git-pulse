APP := git-pulse

.PHONY: build fmt test test-race

build:
	mkdir -p bin
	go build -o bin/$(APP) ./cmd/$(APP)

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

test:
	go test ./...

test-race:
	go test -race ./...
