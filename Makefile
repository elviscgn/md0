.PHONY: build test security bench check clean

build:
	mkdir -p bin
	go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o bin/md0 ./cmd/md0

test:
	go test ./...

security:
	go test ./internal/md0 -run='^TestSecurity' -count=1

bench:
	go test ./internal/md0 -run='^$$' -bench='^BenchmarkRuntime$$' -benchmem -benchtime=50ms -count=1

check: test security
	@test -z "$$(gofmt -l $$(find . -name '*.go' -type f))" || (echo "gofmt required"; gofmt -l $$(find . -name '*.go' -type f); exit 1)
	go vet ./...
	@test -z "$$(go list -m -f '{{if not .Main}}{{.Path}}{{end}}' all)" || (echo "third-party module dependency detected"; exit 1)
	@test -z "$$(go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}' ./... | sort -u)" || (echo "third-party package module detected"; exit 1)

clean:
	rm -rf bin
