package main

import (
	"fmt"
	"strings"
)

type StepMode int

const (
	StepModeNone StepMode = iota
	StepModeSeq
	StepModeParallel
)

func taskSteps(taskName string, def TaskDef, resolve TaskResolver) (StepMode, StepList, bool) {
	if len(def.Parallel) > 0 {
		return StepModeParallel, def.Parallel, true
	}
	if len(def.Seq) > 0 {
		return StepModeSeq, def.Seq, true
	}
	if len(def.Cmd) == 0 {
		return StepModeNone, nil, false
	}
	if len(def.Cmd) > 1 {
		return StepModeSeq, def.Cmd, true
	}
	if len(def.Cmd) == 1 {
		kind, _ := resolveStepKind(def.Cmd[0], resolve)
		if kind == StepTask {
			return StepModeSeq, def.Cmd, true
		}
		return StepModeNone, def.Cmd, false
	}
	return StepModeNone, def.Cmd, false
}

func resolveStepKind(step Step, resolve TaskResolver) (StepKind, string) {
	value := strings.TrimSpace(step.Value)
	switch step.Kind {
	case StepCommand:
		return StepCommand, value
	case StepTask:
		return StepTask, value
	case StepAuto:
		if resolve != nil {
			if _, ok := resolve(value); ok {
				return StepTask, value
			}
		}
		return StepCommand, value
	default:
		return StepCommand, value
	}
}

func stepID(taskName string, mode StepMode, index int) string {
	return fmt.Sprintf("%s::%s::%d", taskName, stepModeName(mode), index)
}

func stepModeName(mode StepMode) string {
	switch mode {
	case StepModeParallel:
		return "par"
	case StepModeSeq:
		return "seq"
	default:
		return "step"
	}
}

func stepPrefix(mode StepMode, index int) string {
	switch mode {
	case StepModeParallel:
		return fmt.Sprintf("%d.", index+1)
	case StepModeSeq:
		return fmt.Sprintf("%d.", index+1)
	default:
		return "-"
	}
}

func stepTaskFromID(target string) (string, bool) {
	parts := strings.SplitN(target, "::", 3)
	if len(parts) != 3 {
		return "", false
	}
	if parts[0] == "" {
		return "", false
	}
	return parts[0], true
}
