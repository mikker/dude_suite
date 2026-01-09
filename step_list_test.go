package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStepListUnmarshalScalar(t *testing.T) {
	var cfg struct {
		Cmd StepList `yaml:"cmd"`
	}

	if err := yaml.Unmarshal([]byte("cmd: echo"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Cmd) != 1 || cfg.Cmd[0].Value != "echo" || cfg.Cmd[0].Kind != StepAuto {
		t.Fatalf("unexpected steps: %#v", cfg.Cmd)
	}
}

func TestStepListUnmarshalCommandMap(t *testing.T) {
	var cfg struct {
		Cmd StepList `yaml:"cmd"`
	}

	if err := yaml.Unmarshal([]byte("cmd: [{cmd: ls}]"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Cmd) != 1 || cfg.Cmd[0].Value != "ls" || cfg.Cmd[0].Kind != StepCommand {
		t.Fatalf("unexpected steps: %#v", cfg.Cmd)
	}
}

func TestStepListUnmarshalCommandMapWithName(t *testing.T) {
	var cfg struct {
		Cmd StepList `yaml:"cmd"`
	}

	if err := yaml.Unmarshal([]byte("cmd: [{cmd: ls, name: list}]"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Cmd) != 1 || cfg.Cmd[0].Value != "ls" || cfg.Cmd[0].Kind != StepCommand || cfg.Cmd[0].Name != "list" {
		t.Fatalf("unexpected steps: %#v", cfg.Cmd)
	}
}

func TestStepListUnmarshalTaskMap(t *testing.T) {
	var cfg struct {
		Seq StepList `yaml:"seq"`
	}

	if err := yaml.Unmarshal([]byte("seq: [{task: build}]"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Seq) != 1 || cfg.Seq[0].Value != "build" || cfg.Seq[0].Kind != StepTask {
		t.Fatalf("unexpected steps: %#v", cfg.Seq)
	}
}

func TestStepListUnmarshalTaskMapWithName(t *testing.T) {
	var cfg struct {
		Seq StepList `yaml:"seq"`
	}

	if err := yaml.Unmarshal([]byte("seq: [{task: build, name: build-all}]"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Seq) != 1 || cfg.Seq[0].Value != "build" || cfg.Seq[0].Kind != StepTask || cfg.Seq[0].Name != "build-all" {
		t.Fatalf("unexpected steps: %#v", cfg.Seq)
	}
}
