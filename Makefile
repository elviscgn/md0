.PHONY: build test check clean

build:
	mkdir -p bin
	go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o bin/md0 ./cmd/md0

test:
	go test ./...

check: test
	go vet ./...
	@test -z "$$(go list -m -f '{{if not .Main}}{{.Path}}{{end}}' all)" || (echo "third-party module dependency detected"; exit 1)

clean:
	rm -rf bin
