.PHONY: test race vet build artifact verify

test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

vet:
	go vet ./...

build:
	go build ./...

artifact:
	./scripts/test-reproducible-artifact.sh

verify: test race vet build artifact
	git diff --check
