.PHONY: test race vet build process artifact verify

test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

vet:
	go vet ./...

build:
	go build ./...

process:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT HUP INT TERM; \
	go build -o "$$tmp/gotth-extension-godaddy-dns" ./cmd/gotth-extension-godaddy-dns; \
	GOTTH_PROVIDER_BINARY="$$tmp/gotth-extension-godaddy-dns" go test -count=1 ./test/process

artifact:
	./scripts/test-reproducible-artifact.sh

verify: test race vet build process artifact
	git diff --check
