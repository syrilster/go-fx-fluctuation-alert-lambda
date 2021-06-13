COVER_FILE?=./gen/coverage.out
COVER_TEXT?=./gen/coverage.txt
COVER_HTML?=./gen/coverage.html
export GO111MODULE=on
export GOFLAGS=-mod=vendor

update-vendor:
	go mod tidy
	go mod vendor

test:
	@mkdir -p gen
	set -eo pipefail; go test -short `go list ./... | grep -vE "./test"` \
	        -race -covermode=atomic -json \
	        -coverprofile=$(COVER_FILE) \
	        | tee $(TEST_JSON)
	go tool cover -func=$(COVER_FILE) \
	        | tee $(COVER_TEXT)
	go tool cover -html=$(COVER_FILE) -o $(COVER_HTML)

lint:
	golangci-lint run -v

PHONY: \
	clean \
	build \
	test \
	container \
	push \