package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Title        string      `yaml:"title"`
	SidebarWidth int         `yaml:"sidebar_width"`
	Shell        string      `yaml:"shell"`
	Theme        string      `yaml:"theme"`
	Init         CommandList `yaml:"init"`
	Tasks        []TaskDef   `yaml:"tasks"`
	Combos       []ComboDef  `yaml:"combos"`
}

type TaskDef struct {
	Name     string   `yaml:"name"`
	Key      string   `yaml:"key"`
	Hidden   bool     `yaml:"hidden"`
	Cmd      StepList `yaml:"cmd"`
	Parallel StepList `yaml:"parallel"`
	Seq      StepList `yaml:"seq"`
}

type ComboDef struct {
	Name       string   `yaml:"name"`
	Key        string   `yaml:"key"`
	Mode       string   `yaml:"mode"` // parallel | sequential
	Run        []string `yaml:"run"`
	StopOnFail *bool    `yaml:"stop_on_fail"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg.normalize(path)
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) normalize(path string) {
	if c.Title == "" {
		title := ""
		if cwd, err := os.Getwd(); err == nil {
			title = filepath.Base(cwd)
		}
		if title == "" || title == "." || title == string(filepath.Separator) {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if base == "" {
				base = "suite"
			}
			title = base
		}
		c.Title = title
	}
	if c.SidebarWidth == 0 {
		c.SidebarWidth = 32
	}
	c.Shell = strings.TrimSpace(c.Shell)
	c.Theme = strings.ToLower(strings.TrimSpace(c.Theme))
	if c.Shell == "" {
		c.Shell = strings.TrimSpace(os.Getenv("SHELL"))
		if c.Shell == "" {
			c.Shell = "/bin/sh"
		}
	}
	c.Init = normalizeCommandList(c.Init)

	for i := range c.Tasks {
		t := &c.Tasks[i]
		t.Key = strings.TrimSpace(t.Key)
		t.Name = strings.TrimSpace(t.Name)
		t.Cmd = normalizeStepList(t.Cmd)
		t.Parallel = normalizeStepList(t.Parallel)
		t.Seq = normalizeStepList(t.Seq)
		if t.Name == "" {
			t.Name = defaultTaskName(*t)
		}
	}

	for i := range c.Combos {
		cb := &c.Combos[i]
		cb.Key = strings.TrimSpace(cb.Key)
		cb.Name = strings.TrimSpace(cb.Name)
		cb.Mode = strings.ToLower(strings.TrimSpace(cb.Mode))
		if cb.Mode == "" {
			cb.Mode = "sequential"
		}
		if cb.Name == "" {
			cb.Name = defaultComboName(*cb)
		}
	}
}

func (c Config) validate() error {
	if len(c.Tasks) == 0 {
		return fmt.Errorf("config must define at least one task")
	}
	if hasEmptyCommand(c.Init) {
		return fmt.Errorf("init commands must be non-empty")
	}
	if c.Theme != "" && c.Theme != "auto" && c.Theme != "light" && c.Theme != "dark" {
		return fmt.Errorf("theme must be one of auto, light, or dark")
	}

	taskNames := map[string]struct{}{}
	keyUsed := map[string]string{}

	for _, t := range c.Tasks {
		if t.Key != "" && len([]rune(t.Key)) != 1 {
			return fmt.Errorf("task key %q must be a single character", t.Key)
		}
		if len(t.Cmd) == 0 && len(t.Parallel) == 0 && len(t.Seq) == 0 {
			return fmt.Errorf("task %q is missing cmd", t.Name)
		}
		if len(t.Cmd) > 0 && len(t.Parallel) > 0 || len(t.Cmd) > 0 && len(t.Seq) > 0 || len(t.Parallel) > 0 && len(t.Seq) > 0 {
			return fmt.Errorf("task %q cannot define both cmd and parallel", t.Name)
		}
		if hasEmptyStep(t.Cmd) || hasEmptyStep(t.Parallel) || hasEmptyStep(t.Seq) {
			return fmt.Errorf("task %q has empty commands", t.Name)
		}
		if t.Name == "" {
			return fmt.Errorf("task name is required")
		}
		if _, ok := taskNames[t.Name]; ok {
			return fmt.Errorf("duplicate task name %q", t.Name)
		}
		taskNames[t.Name] = struct{}{}

		if t.Key != "" {
			if prev, ok := keyUsed[t.Key]; ok {
				return fmt.Errorf("key %q already assigned to %s", t.Key, prev)
			}
			keyUsed[t.Key] = fmt.Sprintf("task %q", t.Name)
		}
	}

	comboNames := map[string]struct{}{}
	for _, cb := range c.Combos {
		if cb.Key == "" {
			return fmt.Errorf("combo %q is missing key", cb.Name)
		}
		if len([]rune(cb.Key)) != 1 {
			return fmt.Errorf("combo %q key must be a single character", cb.Name)
		}
		if len(cb.Run) == 0 {
			return fmt.Errorf("combo %q has no tasks", cb.Name)
		}
		if cb.Mode != "parallel" && cb.Mode != "sequential" {
			return fmt.Errorf("combo %q has invalid mode %q", cb.Name, cb.Mode)
		}
		if cb.Name == "" {
			return fmt.Errorf("combo name is required")
		}
		if _, ok := comboNames[cb.Name]; ok {
			return fmt.Errorf("duplicate combo name %q", cb.Name)
		}
		comboNames[cb.Name] = struct{}{}
		if prev, ok := keyUsed[cb.Key]; ok {
			return fmt.Errorf("key %q already assigned to %s", cb.Key, prev)
		}
		keyUsed[cb.Key] = fmt.Sprintf("combo %q", cb.Name)

		for _, name := range cb.Run {
			if _, ok := taskNames[name]; !ok {
				return fmt.Errorf("combo %q references unknown task %q", cb.Name, name)
			}
		}
	}

	for _, t := range c.Tasks {
		if err := validateTaskStepRefs(t, taskNames); err != nil {
			return err
		}
	}

	return nil
}

func validateTaskStepRefs(t TaskDef, taskNames map[string]struct{}) error {
	for _, step := range t.Cmd {
		if step.Kind == StepTask {
			if _, ok := taskNames[step.Value]; !ok {
				return fmt.Errorf("task %q references unknown task %q", t.Name, step.Value)
			}
		}
	}
	for _, step := range t.Parallel {
		if step.Kind == StepTask {
			if _, ok := taskNames[step.Value]; !ok {
				return fmt.Errorf("task %q references unknown task %q", t.Name, step.Value)
			}
		}
	}
	for _, step := range t.Seq {
		if step.Kind == StepTask {
			if _, ok := taskNames[step.Value]; !ok {
				return fmt.Errorf("task %q references unknown task %q", t.Name, step.Value)
			}
		}
	}
	return nil
}

func stopOnFail(cb ComboDef) bool {
	if cb.StopOnFail == nil {
		return true
	}
	return *cb.StopOnFail
}

func normalizeCommandList(list CommandList) CommandList {
	if len(list) == 0 {
		return nil
	}
	out := make(CommandList, 0, len(list))
	for _, cmd := range list {
		out = append(out, strings.TrimSpace(cmd))
	}
	return out
}

func hasEmptyCommand(list CommandList) bool {
	for _, cmd := range list {
		if strings.TrimSpace(cmd) == "" {
			return true
		}
	}
	return false
}

func normalizeStepList(list StepList) StepList {
	if len(list) == 0 {
		return nil
	}
	out := make(StepList, 0, len(list))
	for _, step := range list {
		step.Value = strings.TrimSpace(step.Value)
		step.Name = strings.TrimSpace(step.Name)
		out = append(out, step)
	}
	return out
}

func hasEmptyStep(list StepList) bool {
	for _, step := range list {
		if strings.TrimSpace(step.Value) == "" {
			return true
		}
	}
	return false
}

func defaultTaskName(t TaskDef) string {
	if len(t.Cmd) > 0 {
		return summarizeSteps(t.Cmd)
	}
	if len(t.Parallel) > 0 {
		return summarizeSteps(t.Parallel)
	}
	if len(t.Seq) > 0 {
		return summarizeSteps(t.Seq)
	}
	if t.Key != "" {
		return t.Key
	}
	return "task"
}

func defaultComboName(c ComboDef) string {
	if c.Name != "" {
		return c.Name
	}
	if c.Key != "" {
		return c.Key
	}
	return "combo"
}

func summarizeSteps(steps []Step) string {
	if len(steps) == 0 {
		return ""
	}
	first := stepDisplayName(steps[0])
	if len(steps) == 1 {
		return first
	}
	return fmt.Sprintf("%s (+%d)", first, len(steps)-1)
}

func stepDisplayName(step Step) string {
	name := strings.TrimSpace(step.Name)
	if name != "" {
		return name
	}
	return strings.TrimSpace(step.Value)
}
