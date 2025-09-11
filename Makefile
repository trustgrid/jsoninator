
run:
	go run main.go

test:
	go test -race -v -timeout 20s ./...

lint:
	golangci-lint run --tests=false ./...