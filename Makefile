run:
	BRANCH=dev go run --race . --url sldent.local::localhost:5173 --url api.sldent.local::example.com
build:
	CGO_ENABLED=0 go build -o ./bin/reverse_mac .
build_server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/reverse_server .