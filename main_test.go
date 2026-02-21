package main

import "testing"

func TestDisableTaskAutostart(t *testing.T) {
	cfg := Config{
		Tasks: []TaskDef{
			{Name: "watch", Autostart: true},
			{Name: "build", Autostart: false},
		},
	}

	disableTaskAutostart(&cfg)

	for _, task := range cfg.Tasks {
		if task.Autostart {
			t.Fatalf("expected task %q autostart disabled", task.Name)
		}
	}
}
