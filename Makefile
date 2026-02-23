.PHONY: build test vet clean

build:
	go build -o gra ./cmd/gra/

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f gra
