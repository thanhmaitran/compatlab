.PHONY: test vet fmt check build-cli build-api docker-up
test:
	go test ./... -race -count=1
vet:
	go vet ./...
fmt:
	gofmt -w .
check: vet test
build-cli:
	go build -trimpath -o bin/compatlab ./apps/cli/cmd/compatlab
build-api:
	go build -trimpath -o bin/server ./apps/api/cmd/server
docker-up:
	docker compose up --build
