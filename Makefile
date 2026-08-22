GO = go
OUTPUT ?= slax
LDFLAGS = -s -w -buildid=
GCFLAGS = all=-l

.PHONY: build check clean fmt

build:
	CGO_ENABLED=0 $(GO) build -mod=readonly -trimpath -buildvcs=false -gcflags='$(GCFLAGS)' -ldflags='$(LDFLAGS)' -o $(OUTPUT) .

fmt:
	$(GO) fmt ./...

check:
	test -z "$$(gofmt -l .)"
	$(GO) mod tidy -diff
	$(GO) vet ./...
	$(GO) test ./...
	$(MAKE) build

clean:
	rm -f $(OUTPUT)
