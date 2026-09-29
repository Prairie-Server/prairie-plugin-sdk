# Contributing to the Prairie Plugin SDK

The [Prairie contribution guide](https://github.com/prairie-server/prairie-server/blob/main/CONTRIBUTING.md)
covers project-wide coordination, focused changes, evidence, AI disclosure, and
pull request expectations. Those requirements apply here; this guide adds the
SDK-specific workflow.

## Before you start

Open an [issue](https://github.com/prairie-server/prairie-plugin-sdk/issues) before
adding or changing a capability, protobuf contract, manifest field, runtime
behavior, or public Go API. The SDK is a versioned contract consumed by
`prairie-server` and every plugin, so identify affected downstream repositories and
the intended compatibility strategy before implementation.

Host-only behavior belongs in
[`prairie-server`](https://github.com/prairie-server/prairie-server); provider-specific
behavior belongs in the individual plugin repository.

## Development setup

Use the Go version declared in `go.mod`. Protobuf regeneration also requires
`protoc`; `make proto` installs the generator tools under `bin/` as needed.
Read [docs/compatibility.md](docs/compatibility.md) before changing a public
contract.

## Validate your change

Run focused package tests while iterating. Before opening a pull request, run:

```sh
go test ./...
go vet ./...
go build ./examples/hello-scheduled-task
go build ./examples/hello-runtime-host
go build ./examples/hello-network-access
gofmt -l .
golangci-lint run ./...
go test $(go list ./pkg/pluginsdk/... | grep -vE '/(runtimedefault|runtime)$') -count=1 -covermode=atomic -coverprofile=coverage.out
./scripts/check-coverage.sh coverage.out
```

CI runs golangci-lint v2.14.0 and enforces a 95% statement coverage floor
(`scripts/check-coverage.sh`) over `pkg/pluginsdk/...`, excluding the
`runtime` and `runtimedefault` wiring packages; the last three commands
reproduce those checks.

For protobuf or generated-code changes, also run:

```sh
make proto
```

`gofmt -l .` should print nothing. If it reports unrelated pre-existing drift,
none of the Go files touched by your change may appear in the output; do not add
to the output, and report what remains. When no contract change is intended,
`make proto` must leave generated code unchanged. Add compatibility coverage
for presence semantics, validation, and existing consumers when a contract
changes.

## Open the pull request

Use a Conventional Commit title, explain the compatibility and release impact,
list downstream coordination, and paste the actual validation results. Read the
[AI-assisted contribution policy](https://github.com/prairie-server/prairie-server/blob/main/docs/ai-contributions.md)
and include its disclosure block.
