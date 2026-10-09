.PHONY: fmt fmt-check vet test build check coverage bench fuzz

fmt:
	gofmt -w $$(find . -type f -name '*.go' -not -path './.git/*')

fmt-check:
	@files=$$(gofmt -l $$(find . -type f -name '*.go' -not -path './.git/*')); \
	if [ -n "$$files" ]; then printf 'Run make fmt for:\n%s\n' "$$files"; exit 1; fi

vet:
	go vet ./...

test:
	go test -race ./...

build:
	go build ./...

check: fmt-check vet test build

coverage:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

bench:
	go test -run='^$$' -bench=. -benchmem .

fuzz:
	go test -run='^$$' -fuzz=FuzzMatchRoute -fuzztime=10s .
