run:
	BRANCH=dev go run --race . --url localhost::example.com
build_server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/reverse_server .