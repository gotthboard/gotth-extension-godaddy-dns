.PHONY: test race vet build format binding licenses source process artifact verify

test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

vet:
	go vet ./...

build:
	go build ./...

format:
	test -z "$$(gofmt -l cmd internal test)"

binding:
	./scripts/check-published-control.sh

licenses:
	./scripts/check-dependency-licenses.sh

source:
	./scripts/check-source-boundary.sh

process:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT HUP INT TERM; \
	go build -o "$$tmp/gotth-extension-godaddy-dns" ./cmd/gotth-extension-godaddy-dns; \
	GOTTH_PROVIDER_BINARY="$$tmp/gotth-extension-godaddy-dns" GOTTH_PROVIDER_SOURCE_ROOT="$$PWD" go test -count=1 ./test/process

artifact:
	./scripts/test-reproducible-artifact.sh

verify: test race vet build format binding licenses source process artifact
	git diff --check
