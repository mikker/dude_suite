# suite

Tiny TUI task runner with hotkeys and a split output view.

## Run

```bash
go run .
```

Or build a binary:

```bash
go build -o suite
./suite
```

By default it reads `.suite.yml` from the current directory. Use `-c` or `--config` to override:

```bash
./suite -c path/to/.suite.yml
```

Theme override (helpful under tmux):

```bash
./suite --theme light
```

## Init

Create a starter config in the current directory:

```bash
./suite init
```

If you run `suite` without a config present, it will prompt to create one.

## Key bindings

- `enter` run selected task
- `up/down` (or `j/k`) move selection
- `ctrl+h` / `ctrl+l` move focus between list and output
- `ctrl+k` kill selected task
- `ctrl+q` quit
- task/combos keys run immediately

When the output pane is focused, scrolling keys control output.

## Config

```yaml
title: suite
sidebar_width: 32
shell: /bin/zsh
theme: auto # auto | light | dark
init:
  - source ~/.zshrc
  - mise activate

tasks:
  - key: f
    name: format
    cmd: bin/format

  - key: t
    name: test
    cmd: bin/test

  - key: a
    name: checks
    parallel:
      - bin/brakeman
      - bin/rubocop
      - bun run prettier . --check

  - key: p
    name: deploy
    cmd:
      - bin/check
      - cmd: git push
      - cmd: git push dokku

  - key: a
    name: full
    seq:
      - format
      - lint
      - test
```

Notes:
- `name` is optional; when omitted it is derived from the command (or key if present).
- `key` is optional; keyless tasks can still be selected and run via enter.
- `hidden: true` hides a task from the main list while keeping it referenceable by other tasks.
- `cmd` can be a single string or a list (sequential).
- `seq` is a list of task names to run sequentially.
- `parallel` runs commands concurrently; `cmd` and `parallel` are mutually exclusive.
- `cmd` and `parallel` list entries can be strings or `{cmd: ...}`. Use `{cmd: ...}` if the string matches a task name.
- Use `{task: name}` to force a task reference when a string would otherwise be treated as a command.
- `shell` (optional) defaults to `$SHELL`. Commands run in that shell with the current environment.
- `theme` (optional) can be `auto`, `light`, or `dark` to override background detection.
- `init` (optional) runs before every command. Use it to source shell setup like `mise activate`.
- Only one instance of a task runs at a time; re-triggering a running task is ignored.
- Only the most recent run output is kept per task.

Combos (optional):

```yaml
combos:
  - name: all
    key: a
    mode: parallel # or sequential
    run: [lint, test]
    # stop_on_fail: false
```

`run` references task names (set `name` explicitly if you want stable references).

## Future ideas

Per-command `cwd`, `env`, `shell`, and timeouts are intentionally omitted in this first version.
