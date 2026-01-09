package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

type TaskOutputMsg struct {
	Target string
	Line   string
}

type TaskStartedMsg struct {
	TaskName string
}

type TaskFinishedMsg struct {
	TaskID   string
	ExitCode int
	Err      error
	Canceled bool
}

type StepStartedMsg struct {
	StepID string
}

type StepFinishedMsg struct {
	StepID   string
	ExitCode int
	Err      error
	Canceled bool
}

type TaskResolver func(name string) (TaskDef, bool)

func listenTaskMsgs(source string, ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return taskStreamMsg{Source: source, Msg: msg}
	}
}

func runTask(ctx context.Context, taskName string, def TaskDef, shell string, init CommandList, resolve TaskResolver, msgCh chan<- tea.Msg) {
	defer close(msgCh)
	stack := map[string]bool{taskName: true}
	_, _ = runTaskInternal(ctx, taskName, def, shell, init, resolve, msgCh, stack)
}

func runTaskInternal(ctx context.Context, taskName string, def TaskDef, shell string, init CommandList, resolve TaskResolver, msgCh chan<- tea.Msg, stack map[string]bool) (int, error) {
	if resolve == nil {
		resolve = func(string) (TaskDef, bool) { return TaskDef{}, false }
	}
	msgCh <- TaskStartedMsg{TaskName: taskName}

	exitCode, err := runTaskSteps(ctx, taskName, def, shell, init, resolve, msgCh, stack)
	msgCh <- TaskFinishedMsg{
		TaskID:   taskName,
		ExitCode: exitCode,
		Err:      err,
		Canceled: ctx.Err() != nil,
	}
	return exitCode, err
}

func runTaskSteps(ctx context.Context, taskName string, def TaskDef, shell string, init CommandList, resolve TaskResolver, msgCh chan<- tea.Msg, stack map[string]bool) (int, error) {
	mode, steps, multi := taskSteps(taskName, def, resolve)
	if !multi {
		if len(steps) == 0 {
			return -1, fmt.Errorf("no commands to run")
		}
		kind, value := resolveStepKind(steps[0], resolve)
		if kind == StepTask {
			child, ok := resolve(value)
			if !ok {
				return -1, fmt.Errorf("unknown task %q", value)
			}
			if stack[value] {
				return -1, fmt.Errorf("task cycle detected at %q", value)
			}
			next := cloneStack(stack)
			next[value] = true
			exitCode, err := runTaskInternal(ctx, value, child, shell, init, resolve, msgCh, next)
			if err != nil {
				return exitCode, err
			}
			return 0, nil
		}
		exitCode, err := runSingle(ctx, steps[0].Value, shell, init, msgCh, taskName)
		if err != nil {
			return exitCode, err
		}
		return 0, nil
	}

	if mode == StepModeParallel {
		return runParallel(ctx, taskName, steps, mode, shell, init, resolve, msgCh, stack)
	}
	return runSequential(ctx, taskName, steps, mode, shell, init, resolve, msgCh, stack)
}

func runSequential(ctx context.Context, taskName string, steps StepList, mode StepMode, shell string, init CommandList, resolve TaskResolver, msgCh chan<- tea.Msg, stack map[string]bool) (int, error) {
	if len(steps) == 0 {
		return -1, fmt.Errorf("no commands to run")
	}

	for idx, step := range steps {
		exitCode, err := runStep(ctx, taskName, step, mode, idx, shell, init, resolve, msgCh, stack)
		if err != nil {
			return exitCode, err
		}
	}

	return 0, nil
}

func runParallel(ctx context.Context, taskName string, steps StepList, mode StepMode, shell string, init CommandList, resolve TaskResolver, msgCh chan<- tea.Msg, stack map[string]bool) (int, error) {
	if len(steps) == 0 {
		return -1, fmt.Errorf("no commands to run")
	}

	type result struct {
		exitCode int
		err      error
	}

	results := make(chan result, len(steps))
	var wg sync.WaitGroup

	for idx, step := range steps {
		step := step
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			exitCode, err := runStep(ctx, taskName, step, mode, idx, shell, init, resolve, msgCh, cloneStack(stack))
			results <- result{exitCode: exitCode, err: err}
		}()
	}

	wg.Wait()
	close(results)

	exitCode := 0
	var err error
	for res := range results {
		if res.err != nil && err == nil {
			err = res.err
			exitCode = res.exitCode
		}
	}

	return exitCode, err
}

func runStep(ctx context.Context, taskName string, step Step, mode StepMode, index int, shell string, init CommandList, resolve TaskResolver, msgCh chan<- tea.Msg, stack map[string]bool) (int, error) {
	value := strings.TrimSpace(step.Value)
	if value == "" {
		return -1, fmt.Errorf("empty step")
	}
	kind, resolved := resolveStepKind(step, resolve)
	switch kind {
	case StepTask:
		def, ok := resolve(resolved)
		if !ok {
			return -1, fmt.Errorf("unknown task %q", resolved)
		}
		if stack[resolved] {
			return -1, fmt.Errorf("task cycle detected at %q", resolved)
		}
		stepID := stepID(taskName, mode, index)
		msgCh <- StepStartedMsg{StepID: stepID}
		next := cloneStack(stack)
		next[resolved] = true
		exitCode, err := runTaskInternal(ctx, resolved, def, shell, init, resolve, msgCh, next)
		msgCh <- StepFinishedMsg{
			StepID:   stepID,
			ExitCode: exitCode,
			Err:      err,
			Canceled: ctx.Err() != nil,
		}
		return exitCode, err
	case StepCommand:
		stepID := stepID(taskName, mode, index)
		msgCh <- StepStartedMsg{StepID: stepID}
		exitCode, err := runSingle(ctx, value, shell, init, msgCh, stepID)
		msgCh <- StepFinishedMsg{
			StepID:   stepID,
			ExitCode: exitCode,
			Err:      err,
			Canceled: ctx.Err() != nil,
		}
		return exitCode, err
	default:
		return -1, fmt.Errorf("unknown step kind")
	}
}

func runSingle(ctx context.Context, command string, shell string, init CommandList, msgCh chan<- tea.Msg, target string) (int, error) {
	fullCommand := buildShellCommand(init, command)
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.CommandContext(ctx, shell, "-c", fullCommand)
	cmd.Env = envWithShell(shell)
	if isTerminal(os.Stdin) {
		cmd.Stdin = os.Stdin
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}

	if err := cmd.Start(); err != nil {
		return -1, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamLines(target, stdout, msgCh, &wg)
	go streamLines(target, stderr, msgCh, &wg)

	err = cmd.Wait()
	wg.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return exitCode, err
}

func buildShellCommand(init CommandList, command string) string {
	command = strings.TrimSpace(command)
	if len(init) == 0 {
		return command
	}
	parts := make([]string, 0, len(init)+1)
	for _, cmd := range init {
		if strings.TrimSpace(cmd) != "" {
			parts = append(parts, cmd)
		}
	}
	if command != "" {
		parts = append(parts, command)
	}
	return strings.Join(parts, "; ")
}

func envWithShell(shell string) []string {
	env := os.Environ()
	if shell == "" {
		return env
	}
	for i, entry := range env {
		if strings.HasPrefix(entry, "SHELL=") {
			env[i] = "SHELL=" + shell
			return env
		}
	}
	return append(env, "SHELL="+shell)
}

func cloneStack(stack map[string]bool) map[string]bool {
	next := make(map[string]bool, len(stack))
	for key, value := range stack {
		next[key] = value
	}
	return next
}

func streamLines(target string, r io.Reader, msgCh chan<- tea.Msg, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		trySend(msgCh, TaskOutputMsg{Target: target, Line: scanner.Text()})
	}

	if err := scanner.Err(); err != nil {
		trySend(msgCh, TaskOutputMsg{Target: target, Line: fmt.Sprintf("[stream error] %v", err)})
	}
}

func trySend(msgCh chan<- tea.Msg, msg tea.Msg) {
	select {
	case msgCh <- msg:
	default:
	}
}
