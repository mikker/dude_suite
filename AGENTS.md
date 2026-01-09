# Repository Guidelines

## Project Structure & Module Organization
- Go sources live in the repo root (for now): `main.go`, `ui.go`, `runner.go`, `config.go`, `step_list.go`, etc.
- Tests use Go’s standard layout alongside sources: `*_test.go` files in the same directory.
- Example config files live at the root: `.suite.yml` and generated templates in `init.go`.
- Documentation lives in `README.md`.

## Build, Test, and Development Commands
- `go run .` — run the TUI from source.
- `go build -o suite` — build a local binary named `suite`.
- `./suite` — run the compiled binary.
- `./suite -c path/to/.suite.yml` — run with a specific config.
- `./suite init` — generate a starter `.suite.yml` in the current directory.
- `go test ./...` — run all unit tests.
- Always run `go build -o suite` after completing work to confirm it builds.

## Coding Style & Naming Conventions
- Go formatting: use `gofmt` on all `.go` files.
- Naming: Go-exported identifiers use `CamelCase`, unexported `camelCase`.
- Keep structs and helpers small; prefer readable, explicit control flow.

## Testing Guidelines
- Framework: Go’s built-in `testing` package.
- Tests live next to the code they validate and use `*_test.go` naming.
- Aim to cover parsing, validation, and execution paths (see `config_test.go`, `runner_test.go`).
- Run `go test ./...` before submitting changes.

## Commit & Pull Request Guidelines
- No commit conventions are established yet; use concise, imperative messages (e.g., “Add seq task support”).
- PRs should include a short summary, testing notes, and config changes if applicable.

## Configuration Tips
- Default config is `.suite.yml` in the working directory.
- `name` is the stable reference for tasks and combos; keys are optional hotkeys.
- Steps can be strings or `{cmd: ...}` / `{task: ...}` for disambiguation.
