package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultConfigName = ".suite.yml"

var isTerminalFn = isTerminal

func runInit(path string) error {
	if path == "" {
		path = defaultConfigName
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	title := "suite"
	if cwd, err := os.Getwd(); err == nil {
		base := filepath.Base(cwd)
		if base != "" && base != "." && base != string(filepath.Separator) {
			title = base
		}
	}

	content := defaultConfigTemplate(title)
	return os.WriteFile(path, []byte(content), 0o644)
}

func defaultConfigTemplate(title string) string {
	if strings.TrimSpace(title) == "" {
		title = "suite"
	}

	return fmt.Sprintf(`title: %q
sidebar_width: 32

tasks:
  - name: format
    key: f
    cmd: bin/format

  - name: test
    key: t
    cmd: bin/test

  - name: checks
    key: a
    parallel:
      - bin/lint
      - bin/test

  - name: deploy
    key: p
    cmd:
      - bin/check
      - git push
`, title)
}

func offerInit(path string) error {
	if !isTerminalFn(os.Stdin) {
		fmt.Fprintf(os.Stderr, "No %s found in this directory. Run `suite init` to create one.\n", path)
		return errors.New("config missing")
	}

	ok, err := promptYesNo(fmt.Sprintf("No %s found. Create one now?", path))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("config missing")
	}

	if err := runInit(path); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Created %s. Edit it, then re-run suite.\n", path)
	return nil
}

func promptYesNo(question string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
