.PHONY: build test vet lint fmt fix clean

build:
	go build -o gra ./cmd/gra/

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

fix:
	go fix ./...

clean:
	rm -f gra
