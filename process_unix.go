//go:build !windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func prepareCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	killProcessGroup(pid, syscall.SIGTERM)
	killDescendants(pid, syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	killProcessGroup(pid, syscall.SIGKILL)
	killDescendants(pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}

func killProcessGroup(pid int, sig syscall.Signal) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil || pgid <= 0 {
		return
	}
	_ = syscall.Kill(-pgid, sig)
}

func killDescendants(root int, sig syscall.Signal) {
	pids, err := descendantPIDs(root)
	if err != nil {
		return
	}
	for _, pid := range pids {
		_ = syscall.Kill(pid, sig)
	}
}

func descendantPIDs(root int) ([]int, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil, err
	}
	children := make(map[int][]int)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		children[ppid] = append(children[ppid], pid)
	}

	descendants := []int{}
	stack := []int{root}
	seen := map[int]bool{root: true}
	for len(stack) > 0 {
		pid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, child := range children[pid] {
			if seen[child] {
				continue
			}
			seen[child] = true
			descendants = append(descendants, child)
			stack = append(stack, child)
		}
	}
	return descendants, nil
}
