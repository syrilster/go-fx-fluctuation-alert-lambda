export GO111MODULE=on
export GOFLAGS=-mod=vendor

update-vendor:
	go mod tidy
	go mod vendor

test:
	go test -v ./... 2>&1 | tee test-output.txt

lint:
	golangci-lint run -v

PHONY: \
	clean \
	build \
	test \
	container \
	push \