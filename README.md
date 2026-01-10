# suite

<img src="https://s3.brnbw.com/CleanShot-2026-01-10-at-22.11.09-2x-qcDLkF7Bil.png" width="1077">

Tiny TUI task runner with hotkeys, split output, and YAML config.

## Features

- Hotkeys + list navigation with expandable task trees.
- Sequential/parallel tasks, task references, and named steps.
- Combos to run multiple tasks in parallel or sequentially.
- Output pane with status bar and prefixed logs for multi-step runs.
- Mouse support (scrolling over output focuses it).
- `suite init` bootstrap + prompt to create a config when missing.
- Theme auto-detection (tmux friendly) with `--theme` override.

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

- `enter` run selected task/step
- `up/down` (or `j/k`) move selection
- `left/right` (or `h/l`) collapse/expand groups
- `tab` toggle focus list/output
- `g` top, `G` bottom (output)
- `q`/`esc` bottom + focus list
- `ctrl+k`/`ctrl+x` kill selected task/step
- `ctrl+q` quit
- `?` help
- task/combos keys run immediately

When output is focused, `pgup/pgdn`, `ctrl+u/d`, arrow keys, and mouse wheel scroll the output.

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
    parallel:
      - cmd: bundle exec rubocop -a
        name: rubocop
      - cmd: bun run prettier . --write
        name: prettier

  - key: t
    name: test
    cmd: bin/test

  - key: a
    name: full
    seq:
      - format
      - name: check_dirty
        cmd: |
          if ! git diff-index --quiet HEAD --; then
            echo "Git repository is not clean. Commit or stash your changes."
            exit 1
          fi
          echo "Git repository is clean."
      - test

  - key: d
    name: deploy
    seq:
      - full
      - cmd: git push
```

Notes:

- `name` is the stable reference for tasks and combos. If omitted it’s derived from the command (or key if present).
- `key` is optional and case-sensitive; keyless tasks can still be selected and run with `enter`.
- `hidden: true` hides a task from the root list while keeping it referenceable by other tasks.
- `cmd` can be a single string or a list (sequential).
- `seq`/`parallel` are lists of steps. Steps can be strings or `{cmd: ...}` / `{task: ...}`.
- Use `{task: name}` to force a task reference when a string would otherwise be treated as a command.
- `shell` (optional) defaults to `$SHELL`. Commands run in that shell with the current environment.
- `init` (optional) runs before every command (useful for `mise activate`).
- Only one instance of a task runs at a time; re-triggering a running task is ignored.
- Only the most recent run output is kept per task/step.

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

## License

MIT
