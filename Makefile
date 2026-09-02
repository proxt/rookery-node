.PHONY: build docker-build test lint clean

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/rookeryd ./cmd/rookeryd

docker-build:
	docker build -t rookery-node .

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/
