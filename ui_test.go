package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMoveSelection(t *testing.T) {
	cfg := Config{
		Tasks: []TaskDef{
			{Name: "a", Key: "a", Cmd: StepList{{Value: "echo", Kind: StepCommand}}},
			{Name: "b", Key: "b", Cmd: StepList{{Value: "echo", Kind: StepCommand}}},
			{Name: "c", Key: "c", Cmd: StepList{{Value: "echo", Kind: StepCommand}}},
		},
		SidebarWidth: 32,
	}

	m := newModel(cfg)
	if m.selected != 0 {
		t.Fatalf("expected selected 0, got %d", m.selected)
	}

	m.moveSelection(-1)
	if m.selected != 0 {
		t.Fatalf("expected selected stay at 0, got %d", m.selected)
	}

	m.moveSelection(1)
	if m.selected != 1 {
		t.Fatalf("expected selected 1, got %d", m.selected)
	}

	m.moveSelection(10)
	if m.selected != 2 {
		t.Fatalf("expected selected last index 2, got %d", m.selected)
	}
}

func TestTaskStatusText(t *testing.T) {
	task := &Task{Status: StatusRunning}
	if taskStatusText(task) != statusIconRunning {
		t.Fatalf("expected running icon")
	}

	task = &Task{Status: StatusSuccess, ExitCode: 0}
	if taskStatusText(task) != statusIconSuccess {
		t.Fatalf("expected success icon")
	}

	task = &Task{Status: StatusFailed, ExitCode: 2}
	if taskStatusText(task) != statusIconFailed+" 2" {
		t.Fatalf("expected fail icon with exit code")
	}

	task = &Task{Status: StatusCanceled}
	if taskStatusText(task) != statusIconCanceled {
		t.Fatalf("expected canceled icon")
	}

	task = &Task{Status: StatusIdle}
	if taskStatusText(task) != "" {
		t.Fatalf("expected empty status")
	}
}

func TestUpdateContinuesListeningAfterChildTaskFinished(t *testing.T) {
	cfg := Config{
		Tasks: []TaskDef{
			{Name: "parent", Cmd: StepList{{Value: "child", Kind: StepAuto}}},
			{Name: "child", Cmd: StepList{{Value: "echo child", Kind: StepCommand}}},
		},
		SidebarWidth: 32,
	}

	m := newModel(cfg)
	ch := make(chan tea.Msg, 1)
	m.streamBySource["parent"] = ch

	_, cmd := m.Update(taskStreamMsg{Source: "parent", Msg: TaskFinishedMsg{TaskID: "child"}})
	if cmd == nil {
		t.Fatalf("expected listen command after child task finished")
	}

	ch <- TaskStartedMsg{TaskName: "parent"}
	msg := cmd()
	stream, ok := msg.(taskStreamMsg)
	if !ok {
		t.Fatalf("expected taskStreamMsg, got %T", msg)
	}
	if stream.Source != "parent" {
		t.Fatalf("expected source parent, got %q", stream.Source)
	}
}

func TestHiddenTasksOmittedFromRoot(t *testing.T) {
	cfg := Config{
		Tasks: []TaskDef{
			{Name: "visible", Cmd: StepList{{Value: "echo", Kind: StepCommand}}},
			{Name: "hidden", Hidden: true, Cmd: StepList{{Value: "echo", Kind: StepCommand}}},
		},
		SidebarWidth: 32,
	}

	m := newModel(cfg)
	if len(m.entries) != 1 {
		t.Fatalf("expected 1 visible entry, got %d", len(m.entries))
	}
	if m.entries[0].Target != "visible" {
		t.Fatalf("expected visible task entry, got %q", m.entries[0].Target)
	}
}

func TestHiddenTaskShowsAsChild(t *testing.T) {
	cfg := Config{
		Tasks: []TaskDef{
			{Name: "parent", Seq: StepList{{Value: "hidden", Kind: StepTask}}},
			{Name: "hidden", Hidden: true, Cmd: StepList{{Value: "echo", Kind: StepCommand}}},
		},
		SidebarWidth: 32,
	}

	m := newModel(cfg)
	m.expanded["task:parent"] = true
	m.rebuildEntries()

	foundHidden := false
	for _, entry := range m.entries {
		if entry.Kind == entryTask && entry.Target == "hidden" {
			foundHidden = true
			break
		}
	}
	if !foundHidden {
		t.Fatalf("expected hidden task to appear as child entry")
	}
}
