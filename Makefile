.PHONY: build run test migrate clean

build:
	go build -o bin/api cmd/api/main.go

run:
	go run cmd/api/main.go

test:
	go test -v ./...

migrate:
	psql -f internal/database/migrations.sql

clean:
	rm -rf bin/
	rm -rf uploads/

docker-build:
	docker build -t cloud-storage-api .

docker-run:
	docker-compose up -d

.PHONY: deps
deps:
	go mod download
	go mod tidy