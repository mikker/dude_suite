package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigTemplate(t *testing.T) {
	content := defaultConfigTemplate("my-app")
	if !strings.Contains(content, "title: \"my-app\"") {
		t.Fatalf("expected title in template, got:\n%s", content)
	}

	content = defaultConfigTemplate(" ")
	if !strings.Contains(content, "title: \"suite\"") {
		t.Fatalf("expected default suite title, got:\n%s", content)
	}
}

func TestRunInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	path := filepath.Join(dir, defaultConfigName)
	if err := runInit(path); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	base := filepath.Base(dir)
	if !strings.Contains(string(data), "title: \""+base+"\"") {
		t.Fatalf("expected title %q in config", base)
	}
}

func TestRunInitExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, defaultConfigName)
	if err := os.WriteFile(path, []byte("title: test\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := runInit(path); err == nil {
		t.Fatalf("expected error for existing file")
	}
}

func TestOfferInitNonTTY(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(silenceOutput(t))
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})

	path := filepath.Join(dir, defaultConfigName)
	if err := offerInit(path); err == nil {
		t.Fatalf("expected error when not a terminal")
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("did not expect config to be created")
	}
}

func TestOfferInitYes(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(silenceOutput(t))
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	origTerminal := isTerminalFn
	isTerminalFn = func(*os.File) bool { return true }
	t.Cleanup(func() { isTerminalFn = origTerminal })

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.Write([]byte("y\n"))
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})

	path := filepath.Join(dir, defaultConfigName)
	if err := offerInit(path); err != nil {
		t.Fatalf("offerInit: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config to be created: %v", err)
	}
}

func TestOfferInitNo(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(silenceOutput(t))
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	origTerminal := isTerminalFn
	isTerminalFn = func(*os.File) bool { return true }
	t.Cleanup(func() { isTerminalFn = origTerminal })

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.Write([]byte("n\n"))
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})

	path := filepath.Join(dir, defaultConfigName)
	if err := offerInit(path); err == nil {
		t.Fatalf("expected error when user declines")
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("did not expect config to be created")
	}
}

func silenceOutput(t *testing.T) func() {
	t.Helper()

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}

	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = devNull
	os.Stderr = devNull

	return func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
		_ = devNull.Close()
	}
}
