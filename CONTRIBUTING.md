# Contributing to MiniRPKI

MiniRPKI is intentionally small: one Go executable, manual VRPs, RTR v1, and
a Containerlab lab. Changes should preserve that model.

Before opening a pull request, read [AGENTS.md](AGENTS.md),
[docs/architecture.md](docs/architecture.md), and
[CONTAINERLAB_GUIDE.md](CONTAINERLAB_GUIDE.md).

## Development checks

Run these from the repository root when Go is installed:

```sh
gofmt -w cmd/mini-rpki/*.go internal/*/*.go
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/mini-rpki
```

The Docker build performs `go test ./...` and creates the normal application
image:

```sh
docker build -t mini-rpki:local .
docker build -t mini-rpki-clab:local lab/containerlab/cache
```

Do not run `./lab/containerlab/lab test` against a lab containing VRPs you
care about. It clears the current dataset and leaves demonstration records.
When changing Containerlab behavior, verify router validation states, not only
TCP connectivity.

## Pull-request expectations

- Keep the Go implementation dependency-free unless a strong reason is
  documented.
- Add or update unit tests for behavior changes.
- Keep documentation accurate for the public `add`, `remove`, `list`,
  `clear`, `status`, and `interface` commands.
- Do not commit `lab/containerlab/data/`, `clab-*`, generated binaries, or
  personal lab topology data.
- Do not claim interoperability with a router vendor that has not been tested.
- Explain protocol changes against RFC 8210 and include wire-level tests where
  appropriate.

This project is for laboratory use. Do not add production-validator claims,
repository-fetching behavior, or cryptographic ROA-validation claims without
an explicit product decision.
