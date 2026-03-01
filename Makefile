.PHONY: build test vet lint fmt clean

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

clean:
	rm -f gra
