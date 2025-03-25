.POSIX:
.SUFFIXES:

gen:
	@go generate ./...

deps:
	@go mod tidy

test:
	@rm -r tests/coverdata tests/coverage.out && mkdir tests/coverdata
	@go test -race -shuffle=on -cover . -args -test.gocoverdir=$$PWD/tests/coverdata
	@$(CONTAINER_RUNNER) compose --file=$$PWD/tests/compose.yaml up --detach
	@go test -v -race -coverpkg=go-simpler.org/queries ./tests -args -test.gocoverdir=$$PWD/tests/coverdata
	@$(CONTAINER_RUNNER) compose --file=$$PWD/tests/compose.yaml down
	@go tool covdata textfmt -i=tests/coverdata -o=tests/coverage.out

test/cover: test
	@go tool cover -html=tests/coverage.out

lint:
	@golangci-lint run
