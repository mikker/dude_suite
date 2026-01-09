package main

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func runTaskAndCollect(ctx context.Context, taskName string, def TaskDef) ([]TaskOutputMsg, TaskFinishedMsg) {
	msgCh := make(chan tea.Msg, 32)
	go runTask(ctx, taskName, def, "/bin/sh", nil, func(string) (TaskDef, bool) { return TaskDef{}, false }, msgCh)

	outputs := []TaskOutputMsg{}
	var done TaskFinishedMsg
	for msg := range msgCh {
		switch msg := msg.(type) {
		case TaskOutputMsg:
			outputs = append(outputs, msg)
		case TaskFinishedMsg:
			if msg.TaskID == taskName {
				done = msg
			}
		}
	}
	return outputs, done
}

func TestRunCommandSuccess(t *testing.T) {
	def := TaskDef{Cmd: StepList{{Value: "printf 'hello\nworld\n'", Kind: StepCommand}}}
	outputs, done := runTaskAndCollect(context.Background(), "task", def)

	if done.Err != nil {
		t.Fatalf("expected no error, got %v", done.Err)
	}
	if done.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", done.ExitCode)
	}

	lines := []string{}
	for _, out := range outputs {
		lines = append(lines, out.Line)
	}
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected output: %v", lines)
	}
}

func TestRunCommandFailure(t *testing.T) {
	def := TaskDef{Cmd: StepList{{Value: "exit 2", Kind: StepCommand}}}
	_, done := runTaskAndCollect(context.Background(), "task", def)

	if done.Err == nil {
		t.Fatalf("expected error")
	}
	if done.ExitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", done.ExitCode)
	}
}

func TestRunCommandCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	def := TaskDef{Cmd: StepList{{Value: "sleep 2", Kind: StepCommand}}}

	msgCh := make(chan tea.Msg, 16)
	go runTask(ctx, "task", def, "/bin/sh", nil, func(string) (TaskDef, bool) { return TaskDef{}, false }, msgCh)

	time.Sleep(100 * time.Millisecond)
	cancel()

	var done TaskFinishedMsg
	for msg := range msgCh {
		if finished, ok := msg.(TaskFinishedMsg); ok && finished.TaskID == "task" {
			done = finished
		}
	}

	if !done.Canceled {
		t.Fatalf("expected canceled true")
	}
}

func TestRunTaskSequentialStopsOnFail(t *testing.T) {
	def := TaskDef{Cmd: StepList{
		{Value: "printf 'one\n'", Kind: StepCommand},
		{Value: "exit 3", Kind: StepCommand},
		{Value: "printf 'two\n'", Kind: StepCommand},
	}}

	outputs, done := runTaskAndCollect(context.Background(), "task", def)

	if done.ExitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", done.ExitCode)
	}
	lines := []string{}
	for _, out := range outputs {
		lines = append(lines, out.Line)
	}
	if len(lines) == 0 || lines[0] != "one" {
		t.Fatalf("expected first line one, got %v", lines)
	}
	for _, line := range lines {
		if line == "two" {
			t.Fatalf("unexpected output after failure")
		}
	}
}

func TestRunTaskParallel(t *testing.T) {
	def := TaskDef{Parallel: StepList{
		{Value: "printf 'alpha\n'", Kind: StepCommand},
		{Value: "printf 'beta\n'", Kind: StepCommand},
	}}

	outputs, done := runTaskAndCollect(context.Background(), "task", def)

	if done.Err != nil {
		t.Fatalf("expected no error, got %v", done.Err)
	}
	if done.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", done.ExitCode)
	}

	found := map[string]bool{}
	for _, out := range outputs {
		found[out.Line] = true
	}
	if !found["alpha"] || !found["beta"] {
		t.Fatalf("expected output from both commands, got %v", outputs)
	}
}

func TestBuildShellCommand(t *testing.T) {
	init := CommandList{"export FOO=bar", "source ~/.zshrc"}
	cmd := buildShellCommand(init, "echo $FOO")
	if !strings.Contains(cmd, "export FOO=bar") || !strings.Contains(cmd, "echo $FOO") {
		t.Fatalf("unexpected command: %s", cmd)
	}
}

func TestRunTaskWithTaskReference(t *testing.T) {
	tasks := map[string]TaskDef{
		"child": {Name: "child", Cmd: StepList{{Value: "printf 'child\n'", Kind: StepCommand}}},
		"parent": {Name: "parent", Cmd: StepList{{Value: "child", Kind: StepAuto}, {Value: "printf 'parent\n'", Kind: StepCommand}}},
	}

	resolve := func(name string) (TaskDef, bool) {
		def, ok := tasks[name]
		return def, ok
	}

	msgCh := make(chan tea.Msg, 16)
	go runTask(context.Background(), "parent", tasks["parent"], "/bin/sh", nil, resolve, msgCh)

	lines := []string{}
	for msg := range msgCh {
		if out, ok := msg.(TaskOutputMsg); ok {
			lines = append(lines, out.Line)
		}
	}

	if len(lines) != 2 || lines[0] != "child" || lines[1] != "parent" {
		t.Fatalf("unexpected output: %v", lines)
	}
}
