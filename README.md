# queries

[![checks](https://github.com/go-simpler/queries/actions/workflows/checks.yml/badge.svg)](https://github.com/go-simpler/queries/actions/workflows/checks.yml)
[![pkg.go.dev](https://pkg.go.dev/badge/go-simpler.org/queries.svg)](https://pkg.go.dev/go-simpler.org/queries)
[![goreportcard](https://goreportcard.com/badge/go-simpler.org/queries)](https://goreportcard.com/report/go-simpler.org/queries)
[![codecov](https://codecov.io/gh/go-simpler/queries/branch/main/graph/badge.svg)](https://codecov.io/gh/go-simpler/queries)

[WIP] Convenience helpers for working with SQL queries.

## Features

- `Builder`: an `fmt`-based query builder with an API similar to `strings.Builder`.
- `Scanner`: a query-to-struct scanner, a lightweight version of `sqlx` with a smaller and stricter API.

## Usage

See [examples](example_test.go).
