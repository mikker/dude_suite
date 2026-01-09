package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCommandListUnmarshalScalar(t *testing.T) {
	var cfg struct {
		Cmd CommandList `yaml:"cmd"`
	}

	if err := yaml.Unmarshal([]byte("cmd: echo"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Cmd) != 1 || cfg.Cmd[0] != "echo" {
		t.Fatalf("unexpected commands: %v", cfg.Cmd)
	}
}

func TestCommandListUnmarshalList(t *testing.T) {
	var cfg struct {
		Cmd CommandList `yaml:"cmd"`
	}

	if err := yaml.Unmarshal([]byte("cmd: [echo, ls]"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Cmd) != 2 || cfg.Cmd[0] != "echo" || cfg.Cmd[1] != "ls" {
		t.Fatalf("unexpected commands: %v", cfg.Cmd)
	}
}
