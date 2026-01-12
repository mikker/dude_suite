//go:build windows

package main

import "os/exec"

func prepareCommand(cmd *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
