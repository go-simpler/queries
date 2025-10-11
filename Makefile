.POSIX:
.SUFFIXES:

bench:
	@go test -run='^$$' -bench=. -cpuprofile=profile.cpu -memprofile=profile.mem

clean:
	@rm -rf tests/coverdata tests/coverage.out tests/test.sqlite

deps:
	@go mod tidy
	@cd tests && go mod tidy

fmt:
	@golangci-lint fmt

gen:
	@go generate ./...

lint:
	@golangci-lint run

test: test/unit test/integration

test/unit: clean
	@mkdir -p tests/coverdata
	@go test -race -shuffle=on -cover . -args -test.gocoverdir=$$PWD/tests/coverdata

test/integration: clean
	@mkdir -p tests/coverdata
	@$(CONTAINER_RUNNER) compose --file=tests/compose.yaml up --detach
	@go test -v -race -coverpkg=go-simpler.org/queries ./tests -args -test.gocoverdir=$$PWD/tests/coverdata
	@$(CONTAINER_RUNNER) compose --file=tests/compose.yaml down
	@go tool covdata textfmt -i=tests/coverdata -o=tests/coverage.out

test/cover: test
	@go tool cover -html=tests/coverage.out
