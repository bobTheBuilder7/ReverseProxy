run:
	BRANCH=dev go run --race . --url localhost::viewer.ohif.org
build:
	CGO_ENABLED=0 go build -o ./bin/reverse_mac .
build_server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/reverse_server .