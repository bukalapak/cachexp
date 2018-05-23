test:
	go test -race -v ./...
coverage:
	go test -race -v -cover -coverprofile=coverage.out ./...
cover:
	go tool cover -html=coverage.out