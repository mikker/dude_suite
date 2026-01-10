package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yml")
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	data := `tasks:
  - name: build
    key: B
    cmd: make build

combos:
  - name: all
    key: A
    run: [build]
`

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	expectedTitle := filepath.Base(dir)
	if cfg.Title != expectedTitle {
		t.Fatalf("expected title %q, got %q", expectedTitle, cfg.Title)
	}
	if cfg.SidebarWidth != 32 {
		t.Fatalf("expected sidebar width 32, got %d", cfg.SidebarWidth)
	}
	if cfg.Shell == "" {
		t.Fatalf("expected default shell to be set")
	}
	if cfg.Tasks[0].Key != "B" {
		t.Fatalf("expected task key preserved, got %q", cfg.Tasks[0].Key)
	}
	if cfg.Tasks[0].Name != "build" {
		t.Fatalf("expected task name, got %q", cfg.Tasks[0].Name)
	}
	if cfg.Combos[0].Key != "A" {
		t.Fatalf("expected combo key preserved, got %q", cfg.Combos[0].Key)
	}
	if cfg.Combos[0].Mode != "sequential" {
		t.Fatalf("expected combo mode default sequential, got %q", cfg.Combos[0].Mode)
	}
	if cfg.Combos[0].Name != "all" {
		t.Fatalf("expected combo name, got %q", cfg.Combos[0].Name)
	}
}

func TestConfigValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "no tasks",
			cfg:  Config{},
		},
		{
			name: "duplicate task name",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}, {Name: "a", Key: "b", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}}},
		},
		{
			name: "duplicate key",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}, {Name: "b", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}}},
		},
		{
			name: "key length",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "aa", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}}},
		},
		{
			name: "missing cmd",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a"}}},
		},
		{
			name: "both cmd and parallel",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}, Parallel: StepList{{Value: "echo", Kind: StepCommand}}}}},
		},
		{
			name: "empty command",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "", Kind: StepCommand}}}}},
		},
		{
			name: "combo unknown task",
			cfg: Config{
				Tasks:  []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}},
				Combos: []ComboDef{{Name: "c", Key: "c", Mode: "sequential", Run: []string{"missing"}}},
			},
		},
		{
			name: "combo invalid mode",
			cfg: Config{
				Tasks:  []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}},
				Combos: []ComboDef{{Name: "c", Key: "c", Mode: "weird", Run: []string{"a"}}},
			},
		},
		{
			name: "combo key length",
			cfg: Config{
				Tasks:  []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}},
				Combos: []ComboDef{{Name: "c", Key: "cc", Mode: "sequential", Run: []string{"a"}}},
			},
		},
		{
			name: "init empty command",
			cfg: Config{
				Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}},
				Init:  CommandList{""},
			},
		},
		{
			name: "combo duplicate name",
			cfg: Config{
				Tasks:  []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}}},
				Combos: []ComboDef{{Name: "c", Key: "c", Mode: "sequential", Run: []string{"a"}}, {Name: "c", Key: "d", Mode: "sequential", Run: []string{"a"}}},
			},
		},
		{
			name: "both cmd and seq",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}, Seq: StepList{{Value: "b", Kind: StepAuto}}}}},
		},
		{
			name: "both seq and parallel",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Seq: StepList{{Value: "b", Kind: StepAuto}}, Parallel: StepList{{Value: "echo", Kind: StepCommand}}}}},
		},
		{
			name: "task step unknown",
			cfg:  Config{Tasks: []TaskDef{{Name: "a", Key: "a", Cmd: StepList{{Value: "missing", Kind: StepTask}}}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.normalize("tasks.yml")
			if err := tc.cfg.validate(); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestStopOnFailDefault(t *testing.T) {
	if !stopOnFail(ComboDef{}) {
		t.Fatalf("expected stopOnFail default true")
	}

	f := false
	if stopOnFail(ComboDef{StopOnFail: &f}) {
		t.Fatalf("expected stopOnFail false")
	}
}

func TestConfigSampleWorks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.yml")
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	data := `tasks:
  - key: l
    name: lint
    parallel:
      - cmd: bin/rubocop
      - cmd: bun run prettier . --check
      - cmd: bun run herb:check
      - cmd: bin/i18n-tasks health

  - key: f
    name: format
    parallel:
      - cmd: rubocop -a
      - cmd: prettier . --write
      - cmd: bun run herb:format
      - cmd: bin/i18n-tasks normalize

  - key: t
    name: test
    cmd: bin/test

  - key: s
    name: security
    parallel:
      - cmd: bin/brakeman -q --no-summary
      - cmd: bin/bundle-audit check --update

  - key: a
    name: full
    seq:
      - format
      - lint
      - test

  - key: p
    cmd:
      - full
      - cmd: git push
      - cmd: git push dokku
`

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Tasks) != 6 {
		t.Fatalf("expected 6 tasks, got %d", len(cfg.Tasks))
	}
	if len(cfg.Tasks[0].Parallel) != 4 {
		t.Fatalf("expected lint task to have 4 commands, got %d", len(cfg.Tasks[0].Parallel))
	}
	if len(cfg.Tasks[5].Cmd) != 3 {
		t.Fatalf("expected p task to have 3 steps, got %d", len(cfg.Tasks[5].Cmd))
	}
	if len(cfg.Tasks[4].Seq) != 3 {
		t.Fatalf("expected full task to have 3 seq steps, got %d", len(cfg.Tasks[4].Seq))
	}
}

func TestKeylessTaskAllowed(t *testing.T) {
	cfg := Config{
		Tasks: []TaskDef{{Name: "build", Cmd: StepList{{Value: "make build", Kind: StepCommand}}}},
	}
	cfg.normalize("tasks.yml")
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected keyless task to be valid, got %v", err)
	}
}

func TestThemeValidation(t *testing.T) {
	cfg := Config{
		Theme: "light",
		Tasks: []TaskDef{{Name: "build", Cmd: StepList{{Value: "make build", Kind: StepCommand}}}},
	}
	cfg.normalize("tasks.yml")
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected theme to be valid, got %v", err)
	}

	cfg.Theme = "neon"
	if err := cfg.validate(); err == nil {
		t.Fatalf("expected invalid theme error")
	}
}
