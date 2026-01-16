package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type TaskStatus int

const (
	StatusIdle TaskStatus = iota
	StatusRunning
	StatusSuccess
	StatusFailed
	StatusCanceled
)

type Task struct {
	Def         TaskDef
	Status      TaskStatus
	Output      []string
	ExitCode    int
	Running     bool
	RunSeq      int
	Steps       []TaskStep
	StepRuns    map[string]*StepRun
	StepTargets map[string]stepTargetInfo
	cancel      context.CancelFunc
	msgCh       chan tea.Msg
}

type TaskStep struct {
	ID       string
	Label    string
	Kind     StepKind
	TaskName string
	Mode     StepMode
	Index    int
}

type StepRun struct {
	ID       string
	Label    string
	Status   TaskStatus
	Output   []string
	ExitCode int
	Running  bool
	RunSeq   int
}

type stepTargetInfo struct {
	Label   string
	Mode    StepMode
	Index   int
	Kind    StepKind
	HasName bool
}

type focusArea int

const (
	focusList focusArea = iota
	focusOutput
)

type comboRun struct {
	Def       ComboDef
	Index     int
	WaitingOn string
	Pending   int
}

type entryKind int

const (
	entryTask entryKind = iota
	entryStep
)

type entry struct {
	ID         string
	Kind       entryKind
	Target     string
	ParentID   string
	ParentTask string
	RootTask   string
	Label      string
	Mode       StepMode
	Index      int
	IsChild    bool
	Depth      int
	Command    string
}

type taskStreamMsg struct {
	Source string
	Msg    tea.Msg
}

type autostartMsg struct {
	TaskName string
}

type selectionPos struct {
	Line int
	Col  int
}

type outputSelection struct {
	Start selectionPos
	End   selectionPos
}

type viewportBounds struct {
	x      int
	y      int
	width  int
	height int
}

type model struct {
	cfg            Config
	tasks          []*Task
	taskByName     map[string]*Task
	stepByID       map[string]*StepRun
	stepCancel     map[string]context.CancelFunc
	combos         []ComboDef
	comboByName    map[string]ComboDef
	taskKeys       map[string]string
	comboKeys      map[string]string
	combosByTask   map[string][]string
	comboActive    map[string]*comboRun
	selected       int
	focus          focusArea
	viewport       viewport.Model
	width          int
	height         int
	sidebarWidth   int
	runSeq         int
	autoScroll     bool
	entries        []entry
	selectedID     string
	expanded       map[string]bool
	streamBySource map[string]chan tea.Msg
	showCheats     bool
	restartPending map[string]bool
	mouseSelecting bool
	selection      outputSelection
}

func newModel(cfg Config) model {
	tasks := make([]*Task, 0, len(cfg.Tasks))
	taskByName := make(map[string]*Task, len(cfg.Tasks))
	taskKeys := make(map[string]string, len(cfg.Tasks))

	for _, def := range cfg.Tasks {
		t := &Task{Def: def, Status: StatusIdle}
		tasks = append(tasks, t)
		taskByName[def.Name] = t
		if def.Key != "" {
			taskKeys[def.Key] = def.Name
		}
	}

	comboByName := make(map[string]ComboDef, len(cfg.Combos))
	comboKeys := make(map[string]string, len(cfg.Combos))
	combosByTask := make(map[string][]string)
	for _, cb := range cfg.Combos {
		comboByName[cb.Name] = cb
		comboKeys[cb.Key] = cb.Name
		for _, taskID := range cb.Run {
			combosByTask[taskID] = append(combosByTask[taskID], cb.Name)
		}
	}

	vp := viewport.New(0, 0)

	m := model{
		cfg:            cfg,
		tasks:          tasks,
		taskByName:     taskByName,
		stepByID:       make(map[string]*StepRun),
		stepCancel:     make(map[string]context.CancelFunc),
		combos:         cfg.Combos,
		comboByName:    comboByName,
		taskKeys:       taskKeys,
		comboKeys:      comboKeys,
		combosByTask:   combosByTask,
		comboActive:    make(map[string]*comboRun),
		selected:       0,
		focus:          focusList,
		viewport:       vp,
		autoScroll:     true,
		expanded:       make(map[string]bool),
		streamBySource: make(map[string]chan tea.Msg),
		restartPending: make(map[string]bool),
	}
	m.rebuildEntries()
	return m
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, task := range m.tasks {
		if !task.Def.Autostart {
			continue
		}
		taskName := task.Def.Name
		cmds = append(cmds, func() tea.Msg {
			return autostartMsg{TaskName: taskName}
		})
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		m.refreshViewport()
		return m, nil
	case autostartMsg:
		return m, m.startTask(msg.TaskName, true)

	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+z" {
			return m, tea.Suspend
		}
		if m.showCheats {
			switch key {
			case "esc", "q", "?":
				m.showCheats = false
				return m, nil
			case "ctrl+q", "ctrl+c":
				m.killAllTasks()
				return m, tea.Quit
			}
			return m, nil
		}
		switch key {
		case "ctrl+q":
			m.killAllTasks()
			return m, tea.Quit
		case "ctrl+c":
			m.killAllTasks()
			return m, tea.Quit
		case "ctrl+k", "ctrl+x":
			return m, m.killSelectedTask()
		case "ctrl+r":
			return m, m.restartSelectedTask()
		case "ctrl+h":
			m.focus = focusList
			return m, nil
		case "ctrl+l":
			m.focus = focusOutput
			return m, nil
		case "tab":
			if m.focus == focusList {
				m.focus = focusOutput
			} else {
				m.focus = focusList
			}
			return m, nil
		case "?":
			m.showCheats = true
			return m, nil
		case "esc", "q":
			m.focus = focusList
			m.autoScroll = true
			m.viewport.GotoBottom()
			m.refreshViewport()
			return m, nil
		}

		if m.focus == focusOutput {
			switch key {
			case "g", "home":
				m.viewport.GotoTop()
				m.autoScroll = m.viewport.AtBottom()
				return m, nil
			case "G", "end":
				m.viewport.GotoBottom()
				m.autoScroll = true
				return m, nil
			}
		}

		if len(key) == 1 {
			if taskID, ok := m.taskKeys[key]; ok {
				if task := m.taskByName[taskID]; task != nil && task.Running {
					m.selectTaskEntry(taskID)
					return m, nil
				}
				return m, m.startTask(taskID, false)
			}
			if comboID, ok := m.comboKeys[key]; ok {
				return m, m.triggerCombo(comboID)
			}
		}

		if m.focus == focusList {
			switch key {
			case "up", "k":
				m.moveSelection(-1)
				return m, nil
			case "down", "j":
				m.moveSelection(1)
				return m, nil
			case "right", "l":
				m.expandSelected()
				return m, nil
			case "left", "h":
				m.collapseSelected()
				return m, nil
			case "enter":
				entry := m.selectedEntry()
				if entry == nil {
					return m, nil
				}
				if entry.Kind == entryTask {
					return m, m.startTask(entry.Target, false)
				}
				if entry.Kind == entryStep {
					return m, m.startStepEntry(*entry)
				}
				return m, nil
			}
		}

		if m.focus == focusOutput {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.autoScroll = m.viewport.AtBottom()
			return m, cmd
		}

	case tea.MouseMsg:
		if msg.X >= m.sidebarWidth {
			m.focus = focusOutput
			if handled, cmd := m.handleOutputMouseSelection(msg); handled {
				return m, cmd
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.autoScroll = m.viewport.AtBottom()
			return m, cmd
		}
		m.focus = focusList
		m.mouseSelecting = false
		return m, nil

	case taskStreamMsg:
		cmds := []tea.Cmd{}
		switch inner := msg.Msg.(type) {
		case TaskStartedMsg:
			m.handleTaskStarted(inner.TaskName)
		case TaskOutputMsg:
			m.handleOutput(inner)
		case TaskFinishedMsg:
			if inner.TaskID == msg.Source {
				if task := m.taskByName[msg.Source]; task != nil {
					task.msgCh = nil
				}
				delete(m.streamBySource, msg.Source)
			}
			m.handleTaskFinished(inner)
			cmds = append(cmds, m.maybeRestartTask(inner.TaskID))
			m.advanceCombos(inner, &cmds)
			m.rebuildEntries()
		case StepStartedMsg:
			m.handleStepStarted(inner)
		case StepFinishedMsg:
			m.handleStepFinished(inner)
			delete(m.streamBySource, inner.StepID)
		}

		if ch, ok := m.streamBySource[msg.Source]; ok && ch != nil {
			cmds = append(cmds, listenTaskMsgs(msg.Source, ch))
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil
	}

	return m, nil
}

func (m *model) handleOutputMouseSelection(msg tea.MouseMsg) (bool, tea.Cmd) {
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown ||
		msg.Button == tea.MouseButtonWheelLeft || msg.Button == tea.MouseButtonWheelRight {
		return false, nil
	}

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return false, nil
		}
		pos, ok := m.selectionPosForMouse(msg, false)
		if !ok {
			return false, nil
		}
		m.mouseSelecting = true
		m.selection = outputSelection{
			Start: pos,
			End:   pos,
		}
		m.autoScroll = false
		return true, nil
	case tea.MouseActionMotion:
		if !m.mouseSelecting {
			return false, nil
		}
		pos, ok := m.selectionPosForMouse(msg, true)
		if !ok {
			return true, nil
		}
		m.selection.End = pos
		return true, nil
	case tea.MouseActionRelease:
		if !m.mouseSelecting {
			return false, nil
		}
		if m.selection.End == m.selection.Start {
			if pos, ok := m.selectionPosForMouse(msg, true); ok {
				m.selection.End = pos
			}
		}
		text := m.selectionText()
		m.mouseSelecting = false
		if text == "" {
			return true, nil
		}
		return true, copyToClipboardCmd(text)
	}

	return false, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	help := m.renderHelp()
	mainHeight := m.height - lipgloss.Height(help)
	if mainHeight < 1 {
		return help
	}

	sidebar := m.renderSidebar(mainHeight)
	output := m.renderOutput(mainHeight)
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, output)

	view := lipgloss.JoinVertical(lipgloss.Left, main, help)
	base := fitView(view, m.width, m.height)
	if m.showCheats {
		return overlayView(base, m.renderCheatsheet())
	}
	return base
}

func (m *model) setSize(width, height int) {
	// Avoid writing to the bottom row, which can trigger terminal scroll.
	if height > 1 {
		height--
	}
	m.width = width
	m.height = height

	minPaneWidth := 20
	sidebarWidth := m.cfg.SidebarWidth
	if sidebarWidth < minPaneWidth {
		sidebarWidth = minPaneWidth
	}
	if width < minPaneWidth*2 {
		sidebarWidth = width / 2
		if sidebarWidth < 10 {
			sidebarWidth = width / 2
		}
	}
	outputWidth := width - sidebarWidth
	if outputWidth < minPaneWidth {
		outputWidth = minPaneWidth
		sidebarWidth = width - outputWidth
		if sidebarWidth < 10 {
			sidebarWidth = 10
		}
	}
	m.sidebarWidth = sidebarWidth

	helpHeight := lipgloss.Height(m.renderHelp())
	mainHeight := height - helpHeight
	if mainHeight < 5 {
		mainHeight = 5
	}

	frameWidth, frameHeight := outputStyle.GetFrameSize()
	contentWidth := outputWidth - frameWidth
	contentHeight := mainHeight - frameHeight - 3
	if contentWidth < 10 {
		contentWidth = 10
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	innerWidth := contentWidth - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	m.viewport.Width = innerWidth
	m.viewport.Height = contentHeight
}

func (m *model) refreshViewport() {
	entry := m.selectedEntry()
	if entry == nil {
		m.viewport.SetContent("No output yet.")
		return
	}

	lines := m.outputForEntry(*entry)
	if len(lines) == 0 {
		m.viewport.SetContent("No output yet.")
		return
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *model) moveSelection(delta int) {
	if len(m.entries) == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.entries) {
		m.selected = len(m.entries) - 1
	}
	if m.selected >= 0 && m.selected < len(m.entries) {
		m.selectedID = m.entries[m.selected].ID
	}
	m.autoScroll = true
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *model) selectedEntry() *entry {
	if len(m.entries) == 0 || m.selected < 0 || m.selected >= len(m.entries) {
		return nil
	}
	return &m.entries[m.selected]
}

func (m *model) entryExpandable(entry entry) bool {
	if entry.Kind != entryTask {
		return false
	}
	def, ok := m.resolveTask(entry.Target)
	if !ok {
		return false
	}
	_, _, multi := taskSteps(entry.Target, def, m.resolveTask)
	return multi
}

func (m *model) expandSelected() {
	entry := m.selectedEntry()
	if entry == nil {
		return
	}
	if !m.entryExpandable(*entry) {
		return
	}
	if m.expanded[entry.ID] {
		return
	}
	m.expanded[entry.ID] = true
	m.rebuildEntries()
}

func (m *model) collapseSelected() {
	entry := m.selectedEntry()
	if entry == nil {
		return
	}
	if m.entryExpandable(*entry) && m.expanded[entry.ID] {
		m.expanded[entry.ID] = false
		m.rebuildEntries()
		return
	}
	if entry.ParentID != "" {
		m.selectedID = entry.ParentID
		m.rebuildEntries()
		m.refreshViewport()
	}
}

func (m *model) isTaskDisabled(taskName string) bool {
	for _, comboName := range m.combosByTask[taskName] {
		if _, ok := m.comboActive[comboName]; ok {
			return true
		}
	}
	return false
}

func (m *model) isComboDisabled(comboName string) bool {
	if _, ok := m.comboActive[comboName]; ok {
		return true
	}
	cb, ok := m.comboByName[comboName]
	if !ok {
		return true
	}
	for _, taskName := range cb.Run {
		if task, ok := m.taskByName[taskName]; ok && task.Running {
			return true
		}
	}
	return false
}

func (m *model) rebuildEntries() {
	entries := make([]entry, 0, len(m.tasks))
	for _, task := range m.tasks {
		if task.Def.Hidden {
			continue
		}
		root := entry{
			ID:       "task:" + task.Def.Name,
			Kind:     entryTask,
			Target:   task.Def.Name,
			Label:    task.Def.Name,
			RootTask: task.Def.Name,
			Depth:    0,
		}
		entries = append(entries, root)
		m.appendTaskChildren(&entries, root, map[string]bool{})
	}
	m.entries = entries

	if m.selectedID != "" {
		for i, entry := range entries {
			if entry.ID == m.selectedID {
				m.selected = i
				return
			}
		}
	}

	if len(entries) > 0 {
		if m.selected < 0 || m.selected >= len(entries) {
			m.selected = 0
		}
		m.selectedID = entries[m.selected].ID
	} else {
		m.selected = 0
		m.selectedID = ""
	}
}

func (m *model) appendTaskChildren(entries *[]entry, parent entry, stack map[string]bool) {
	if !m.expanded[parent.ID] {
		return
	}
	def, ok := m.resolveTask(parent.Target)
	if !ok {
		return
	}
	if stack[parent.Target] {
		return
	}
	stack[parent.Target] = true

	mode, steps, multi := taskSteps(parent.Target, def, m.resolveTask)
	if !multi {
		return
	}

	for idx, step := range steps {
		kind, value := resolveStepKind(step, m.resolveTask)
		label := stepDisplayName(step)
		if kind == StepCommand {
			stepID := stepID(parent.Target, mode, idx)
			*entries = append(*entries, entry{
				ID:         fmt.Sprintf("step:%s:%d", parent.ID, idx),
				Kind:       entryStep,
				Target:     stepID,
				ParentID:   parent.ID,
				ParentTask: parent.Target,
				RootTask:   parent.RootTask,
				Label:      label,
				Mode:       mode,
				Index:      idx,
				Depth:      parent.Depth + 1,
				Command:    value,
			})
			continue
		}

		child := entry{
			ID:         fmt.Sprintf("child:%s:%s", parent.ID, value),
			Kind:       entryTask,
			Target:     value,
			ParentID:   parent.ID,
			ParentTask: parent.Target,
			RootTask:   parent.RootTask,
			Label:      label,
			Mode:       mode,
			Index:      idx,
			IsChild:    true,
			Depth:      parent.Depth + 1,
		}
		*entries = append(*entries, child)
		if m.expanded[child.ID] && !stack[value] {
			next := cloneStack(stack)
			m.appendTaskChildren(entries, child, next)
		}
	}
}

func (m *model) prepareTaskSteps(task *Task) {
	task.Steps = nil
	task.StepTargets = nil

	mode, steps, multi := taskSteps(task.Def.Name, task.Def, m.resolveTask)
	if !multi {
		return
	}

	if task.StepRuns == nil {
		task.StepRuns = make(map[string]*StepRun)
	}
	task.StepTargets = make(map[string]stepTargetInfo)
	for idx, step := range steps {
		kind, value := resolveStepKind(step, m.resolveTask)
		name := strings.TrimSpace(step.Name)
		label := value
		hasName := false
		if name != "" {
			label = name
			hasName = true
		}
		id := stepID(task.Def.Name, mode, idx)
		if kind == StepCommand {
			run, ok := task.StepRuns[id]
			if !ok {
				run = &StepRun{ID: id, Status: StatusIdle}
				task.StepRuns[id] = run
			}
			run.Label = label
			run.Status = StatusIdle
			run.ExitCode = 0
			run.Running = false
			run.RunSeq = task.RunSeq
			m.stepByID[id] = run
			task.StepTargets[id] = stepTargetInfo{
				Label:   label,
				Mode:    mode,
				Index:   idx,
				Kind:    StepCommand,
				HasName: hasName,
			}
			task.Steps = append(task.Steps, TaskStep{
				ID:    id,
				Label: label,
				Kind:  StepCommand,
				Mode:  mode,
				Index: idx,
			})
		} else {
			run, ok := task.StepRuns[id]
			if !ok {
				run = &StepRun{ID: id, Status: StatusIdle}
				task.StepRuns[id] = run
			}
			run.Label = label
			run.Status = StatusIdle
			run.ExitCode = 0
			run.Running = false
			run.RunSeq = task.RunSeq
			m.stepByID[id] = run
			task.StepTargets[value] = stepTargetInfo{
				Label:   label,
				Mode:    mode,
				Index:   idx,
				Kind:    StepTask,
				HasName: hasName,
			}
			task.Steps = append(task.Steps, TaskStep{
				ID:       id,
				Label:    label,
				Kind:     StepTask,
				TaskName: value,
				Mode:     mode,
				Index:    idx,
			})
		}
	}
}

func (m *model) handleTaskStarted(taskName string) {
	task := m.taskByName[taskName]
	if task == nil || (task.Running && task.RunSeq != 0) {
		return
	}
	task.Output = nil
	task.Status = StatusRunning
	task.ExitCode = 0
	task.Running = true
	m.runSeq++
	task.RunSeq = m.runSeq
	m.prepareTaskSteps(task)
	m.resetChildTaskStatuses(task)
	m.updateParentStepRuns(taskName, StatusRunning, 0)
	m.rebuildEntries()
}

func (m *model) handleTaskFinished(msg TaskFinishedMsg) {
	task := m.taskByName[msg.TaskID]
	if task == nil {
		return
	}
	task.Running = false
	task.cancel = nil
	if msg.Canceled {
		task.Status = StatusCanceled
	} else if msg.Err != nil {
		task.Status = StatusFailed
	} else {
		task.Status = StatusSuccess
	}
	task.ExitCode = msg.ExitCode
	task.StepTargets = nil
	m.updateParentStepRuns(msg.TaskID, task.Status, msg.ExitCode)
	m.rebuildEntries()
	if entry := m.selectedEntry(); entry != nil && entry.Kind == entryTask && entry.Target == task.Def.Name {
		m.refreshViewport()
	}
}

func (m *model) handleStepStarted(msg StepStartedMsg) {
	step := m.stepByID[msg.StepID]
	if step == nil {
		return
	}
	if taskName, ok := stepTaskFromID(msg.StepID); ok {
		if task := m.taskByName[taskName]; task != nil {
			step.RunSeq = task.RunSeq
		}
	}
	step.Output = nil
	step.ExitCode = 0
	step.Running = true
	step.Status = StatusRunning
	if entry := m.selectedEntry(); entry != nil && entry.Kind == entryStep && entry.Target == msg.StepID {
		m.refreshViewport()
	}
}

func (m *model) handleStepFinished(msg StepFinishedMsg) {
	step := m.stepByID[msg.StepID]
	if step == nil {
		return
	}
	step.Running = false
	delete(m.stepCancel, msg.StepID)
	if msg.Canceled {
		step.Status = StatusCanceled
	} else if msg.Err != nil {
		step.Status = StatusFailed
	} else {
		step.Status = StatusSuccess
	}
	step.ExitCode = msg.ExitCode
	if entry := m.selectedEntry(); entry != nil && entry.Kind == entryStep && entry.Target == msg.StepID {
		m.refreshViewport()
	}
}

func (m *model) resetChildTaskStatuses(task *Task) {
	for _, step := range task.Steps {
		if step.Kind != StepTask {
			continue
		}
		child := m.taskByName[step.TaskName]
		if child == nil || child.Running {
			continue
		}
		m.resetTaskStatus(child)
	}
}

func (m *model) resetTaskStatus(task *Task) {
	task.Status = StatusIdle
	task.ExitCode = 0
	task.Running = false
	task.RunSeq = 0
	for _, run := range task.StepRuns {
		run.Status = StatusIdle
		run.ExitCode = 0
		run.Running = false
		run.RunSeq = 0
	}
}

func (m *model) updateParentStepRuns(childName string, status TaskStatus, exitCode int) {
	for _, task := range m.tasks {
		if !task.Running || task.RunSeq == 0 {
			continue
		}
		for _, step := range task.Steps {
			if step.Kind != StepTask || step.TaskName != childName {
				continue
			}
			run := task.StepRuns[step.ID]
			if run == nil {
				continue
			}
			run.RunSeq = task.RunSeq
			run.ExitCode = exitCode
			run.Running = status == StatusRunning
			run.Status = status
		}
	}
}

func (m *model) handleOutput(msg TaskOutputMsg) {
	if task := m.taskByName[msg.Target]; task != nil {
		task.Output = append(task.Output, msg.Line)
	}
	if step := m.stepByID[msg.Target]; step != nil {
		step.Output = append(step.Output, msg.Line)
	}

	shouldRefresh := false
	for _, task := range m.tasks {
		if !task.Running || len(task.StepTargets) == 0 {
			continue
		}
		if info, ok := task.StepTargets[msg.Target]; ok {
			prefix := m.stepOutputPrefix(info)
			task.Output = append(task.Output, fmt.Sprintf("%s: %s", prefix, msg.Line))
			if entry := m.selectedEntry(); entry != nil && entry.Kind == entryTask && entry.Target == task.Def.Name {
				shouldRefresh = true
			}
			continue
		}
		if taskName, ok := stepTaskFromID(msg.Target); ok {
			if info, ok := task.StepTargets[taskName]; ok {
				prefix := m.stepOutputPrefix(info)
				task.Output = append(task.Output, fmt.Sprintf("%s: %s", prefix, msg.Line))
				if entry := m.selectedEntry(); entry != nil && entry.Kind == entryTask && entry.Target == task.Def.Name {
					shouldRefresh = true
				}
			}
		}
	}

	if entry := m.selectedEntry(); entry != nil && entry.Target == msg.Target {
		shouldRefresh = true
	}

	if shouldRefresh {
		m.refreshViewport()
		if m.autoScroll {
			m.viewport.GotoBottom()
		}
	}
}

func (m *model) outputForEntry(entry entry) []string {
	if entry.Kind == entryStep {
		if step := m.stepByID[entry.Target]; step != nil {
			return step.Output
		}
		return nil
	}
	if task := m.taskByName[entry.Target]; task != nil {
		return task.Output
	}
	return nil
}

func (m *model) selectionPosForMouse(msg tea.MouseMsg, clamp bool) (selectionPos, bool) {
	bounds, ok := m.outputViewportBounds()
	if !ok {
		return selectionPos{}, false
	}

	x := msg.X
	y := msg.Y
	if clamp {
		if x < bounds.x {
			x = bounds.x
		}
		if x >= bounds.x+bounds.width {
			x = bounds.x + bounds.width - 1
		}
		if y < bounds.y {
			y = bounds.y
		}
		if y >= bounds.y+bounds.height {
			y = bounds.y + bounds.height - 1
		}
	} else if x < bounds.x || x >= bounds.x+bounds.width || y < bounds.y || y >= bounds.y+bounds.height {
		return selectionPos{}, false
	}

	return selectionPos{
		Line: y - bounds.y,
		Col:  x - bounds.x,
	}, true
}

func (m *model) selectionText() string {
	entry := m.selectedEntry()
	if entry == nil {
		return ""
	}
	lines := m.outputForEntry(*entry)
	if len(lines) == 0 {
		return ""
	}

	start, end := normalizeSelection(m.selection.Start, m.selection.End)
	if start.Line == end.Line && start.Col == end.Col {
		return ""
	}

	start.Line += m.viewport.YOffset
	end.Line += m.viewport.YOffset

	maxLine := len(lines) - 1
	start.Line = clampInt(start.Line, 0, maxLine)
	end.Line = clampInt(end.Line, 0, maxLine)
	start.Col = clampInt(start.Col, 0, m.viewport.Width)
	end.Col = clampInt(end.Col, 0, m.viewport.Width)

	endCol := end.Col + 1
	if end.Col >= m.viewport.Width-1 {
		endCol = ansi.StringWidth(lines[end.Line])
	}

	if start.Line == end.Line {
		return trimRightSpaces(cutPlain(lines[start.Line], start.Col, endCol))
	}

	var out []string
	out = append(out, trimRightSpaces(cutPlain(lines[start.Line], start.Col, ansi.StringWidth(lines[start.Line]))))
	for i := start.Line + 1; i < end.Line; i++ {
		out = append(out, trimRightSpaces(ansi.Strip(lines[i])))
	}
	out = append(out, trimRightSpaces(cutPlain(lines[end.Line], 0, endCol)))
	return strings.Join(out, "\n")
}

func (m model) outputViewportBounds() (viewportBounds, bool) {
	if m.width == 0 || m.height == 0 {
		return viewportBounds{}, false
	}

	outputWidth := m.width - m.sidebarWidth
	if outputWidth < 20 {
		outputWidth = 20
	}

	borderWidth, borderHeight := borderSize(outputStyle)
	if outputWidth-borderWidth < 1 {
		return viewportBounds{}, false
	}

	borderLeft := borderWidth / 2
	borderTop := borderHeight / 2
	viewportX := m.sidebarWidth + borderLeft + 1
	viewportY := borderTop + 3

	if m.viewport.Width <= 0 || m.viewport.Height <= 0 {
		return viewportBounds{}, false
	}

	return viewportBounds{
		x:      viewportX,
		y:      viewportY,
		width:  m.viewport.Width,
		height: m.viewport.Height,
	}, true
}

func (m *model) entryStatus(entry entry) (TaskStatus, string) {
	if entry.Kind == entryStep {
		if step := m.stepByID[entry.Target]; step != nil {
			return step.Status, stepStatusText(step)
		}
		return StatusIdle, ""
	}
	if task := m.taskByName[entry.Target]; task != nil {
		return task.Status, taskStatusText(task)
	}
	return StatusIdle, ""
}

func (m *model) entryMarker(entry entry) string {
	if !m.entryExpandable(entry) {
		return " "
	}
	if m.expanded[entry.ID] {
		return "▾"
	}
	return "▸"
}

func stepStatusText(step *StepRun) string {
	return statusLabel(step.Status, step.ExitCode)
}

func (m *model) stepOutputPrefix(info stepTargetInfo) string {
	if info.Kind == StepTask && strings.TrimSpace(info.Label) != "" {
		if info.Mode == StepModeParallel {
			return parallelPrefixStyle(info.Index).Render(info.Label)
		}
		return info.Label
	}
	if info.HasName && strings.TrimSpace(info.Label) != "" {
		if info.Mode == StepModeParallel {
			return parallelPrefixStyle(info.Index).Render(info.Label)
		}
		return info.Label
	}
	if info.Mode == StepModeParallel {
		prefix := fmt.Sprintf("%d", info.Index+1)
		return parallelPrefixStyle(info.Index).Render(prefix)
	}
	if info.Mode == StepModeSeq {
		return fmt.Sprintf("%d", info.Index+1)
	}
	return info.Label
}

func (m *model) renderEntryLine(entry entry) string {
	indent := strings.Repeat("  ", entry.Depth)
	if entry.Kind == entryTask {
		task := m.taskByName[entry.Target]
		base := entry.Label
		if task != nil {
			base = taskDisplayName(task.Def)
			if entry.IsChild && entry.Label != "" && entry.Label != task.Def.Name {
				base = entry.Label
			}
		}
		marker := m.entryMarker(entry)
		line := fmt.Sprintf("%s%s %s", indent, marker, base)
		statusKind, status := m.entryStatus(entry)
		if status == "" {
			return line
		}
		if task != nil && task.Running && task.Def.Persistent {
			return fmt.Sprintf("%s  %s", line, statusStyle(StatusSuccess).Render(status))
		}
		return fmt.Sprintf("%s  %s", line, statusStyle(statusKind).Render(status))
	}

	prefix := stepPrefix(entry.Mode, entry.Index)
	line := fmt.Sprintf("%s%s %s", indent, prefix, entry.Label)
	statusKind, status := m.entryStatus(entry)
	if status == "" {
		return line
	}
	return fmt.Sprintf("%s  %s", line, statusStyle(statusKind).Render(status))
}

func (m *model) startTask(taskName string, fromCombo bool) tea.Cmd {
	task := m.taskByName[taskName]
	if task == nil {
		return nil
	}
	if task.Running {
		return nil
	}
	if !fromCombo && m.isTaskDisabled(taskName) {
		return nil
	}

	task.Output = nil
	task.Status = StatusRunning
	task.ExitCode = 0
	task.Running = true
	m.runSeq++
	task.RunSeq = m.runSeq
	m.prepareTaskSteps(task)
	m.resetChildTaskStatuses(task)

	ctx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel

	msgCh := make(chan tea.Msg, 128)
	task.msgCh = msgCh
	m.streamBySource[taskName] = msgCh

	go runTask(ctx, taskName, task.Def, m.cfg.Shell, m.cfg.Init, m.resolveTask, msgCh)

	m.rebuildEntries()
	if !fromCombo {
		m.selectTaskEntry(taskName)
	}
	entry := m.selectedEntry()
	if entry != nil && entry.Kind == entryTask && entry.Target == taskName {
		m.autoScroll = true
		m.refreshViewport()
		m.viewport.GotoBottom()
	}

	return listenTaskMsgs(taskName, msgCh)
}

func (m *model) startStepEntry(entry entry) tea.Cmd {
	if entry.Kind != entryStep {
		return nil
	}
	command := strings.TrimSpace(entry.Command)
	if command == "" {
		return nil
	}

	stepID := entry.Target
	step := m.stepByID[stepID]
	if step == nil {
		step = &StepRun{ID: stepID, Label: entry.Label, Status: StatusIdle}
		m.stepByID[stepID] = step
	}
	if step.Running {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.stepCancel[stepID] = cancel

	msgCh := make(chan tea.Msg, 128)
	m.streamBySource[stepID] = msgCh

	go func() {
		defer close(msgCh)
		msgCh <- StepStartedMsg{StepID: stepID}
		exitCode, err := runSingle(ctx, command, m.cfg.Shell, m.cfg.Init, msgCh, stepID)
		msgCh <- StepFinishedMsg{
			StepID:   stepID,
			ExitCode: exitCode,
			Err:      err,
			Canceled: ctx.Err() != nil,
		}
	}()

	m.autoScroll = true
	m.refreshViewport()
	m.viewport.GotoBottom()

	return listenTaskMsgs(stepID, msgCh)
}

func (m *model) selectTaskEntry(taskName string) {
	targetID := "task:" + taskName
	for i, entry := range m.entries {
		if entry.ID == targetID {
			m.selected = i
			m.selectedID = targetID
			m.autoScroll = true
			m.refreshViewport()
			m.viewport.GotoBottom()
			return
		}
	}
}

func (m *model) resolveTask(name string) (TaskDef, bool) {
	task := m.taskByName[name]
	if task == nil {
		return TaskDef{}, false
	}
	return task.Def, true
}

func (m *model) killSelectedTask() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	if entry.Kind == entryStep {
		if cancel := m.stepCancel[entry.Target]; cancel != nil {
			cancel()
			return nil
		}
	}

	var task *Task
	if entry.Kind == entryTask {
		task = m.taskByName[entry.Target]
	}
	if task == nil || task.cancel == nil {
		if entry.RootTask != "" {
			task = m.taskByName[entry.RootTask]
		}
	}
	if task != nil && task.cancel != nil {
		task.cancel()
	}
	return nil
}

func (m *model) killAllTasks() {
	for _, cancel := range m.stepCancel {
		if cancel != nil {
			cancel()
		}
	}
	for _, task := range m.taskByName {
		if task != nil && task.cancel != nil {
			task.cancel()
		}
	}
}

func (m *model) restartSelectedTask() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	if entry.Kind == entryTask {
		return m.restartTask(entry.Target)
	}
	if entry.ParentTask != "" {
		return m.restartTask(entry.ParentTask)
	}
	if entry.RootTask != "" {
		return m.restartTask(entry.RootTask)
	}
	return nil
}

func (m *model) restartTask(taskName string) tea.Cmd {
	task := m.taskByName[taskName]
	if task == nil {
		return nil
	}
	if task.Running {
		m.restartPending[taskName] = true
		if task.cancel != nil {
			task.cancel()
		}
		return nil
	}
	return m.startTask(taskName, false)
}

func (m *model) maybeRestartTask(taskName string) tea.Cmd {
	if !m.restartPending[taskName] {
		return nil
	}
	delete(m.restartPending, taskName)
	return m.startTask(taskName, false)
}

func (m *model) triggerCombo(comboName string) tea.Cmd {
	cb, ok := m.comboByName[comboName]
	if !ok {
		return nil
	}
	if m.isComboDisabled(comboName) {
		return nil
	}

	if cb.Mode == "parallel" {
		cmds := make([]tea.Cmd, 0, len(cb.Run))
		run := &comboRun{Def: cb, Pending: len(cb.Run)}
		m.comboActive[cb.Name] = run
		for _, name := range cb.Run {
			cmds = append(cmds, m.startTask(name, true))
		}
		return tea.Batch(cmds...)
	}

	run := &comboRun{Def: cb, Index: 0}
	m.comboActive[cb.Name] = run

	cmd := m.startNextComboTask(run)
	return cmd
}

func (m *model) startNextComboTask(run *comboRun) tea.Cmd {
	for run.Index < len(run.Def.Run) {
		taskName := run.Def.Run[run.Index]
		run.WaitingOn = taskName

		if m.taskByName[taskName].Running {
			return nil
		}
		return m.startTask(taskName, true)
	}
	run.WaitingOn = ""
	return nil
}

func (m *model) advanceCombos(msg TaskFinishedMsg, cmds *[]tea.Cmd) {
	if len(m.comboActive) == 0 {
		return
	}

	comboNames := m.combosByTask[msg.TaskID]
	for _, comboName := range comboNames {
		run, ok := m.comboActive[comboName]
		if !ok {
			continue
		}

		if run.Def.Mode == "parallel" {
			if run.Pending > 0 {
				run.Pending--
			}
			if run.Pending <= 0 {
				delete(m.comboActive, comboName)
			}
			continue
		}

		if run.WaitingOn != msg.TaskID {
			continue
		}

		if msg.Err != nil && stopOnFail(run.Def) {
			delete(m.comboActive, comboName)
			continue
		}

		run.Index++
		if run.Index >= len(run.Def.Run) {
			delete(m.comboActive, comboName)
			continue
		}

		cmd := m.startNextComboTask(run)
		if cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
}

func (m model) renderSidebar(height int) string {
	width := m.sidebarWidth
	if width < 20 {
		width = 20
	}
	borderWidth, borderHeight := borderSize(sidebarStyle)
	contentWidth := width - borderWidth
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := height - borderHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	title := titleStyle.Render(m.cfg.Title)
	lines := []string{title, sectionStyle.Render("Tasks")}

	for i, entry := range m.entries {
		line := m.renderEntryLine(entry)
		disabled := entry.Kind == entryTask && m.isTaskDisabled(entry.Target)
		if disabled && entry.Kind == entryTask {
			line = disabledStyle.Render(line)
		}
		if i == m.selected {
			if disabled {
				line = selectedDisabledStyle.Render(line)
			} else {
				line = selectedStyle.Render(line)
			}
		}
		lines = append(lines, line)
	}

	if len(m.combos) > 0 {
		lines = append(lines, "", sectionStyle.Render("Combos"))
		for _, cb := range m.combos {
			line := renderComboLine(cb, m.comboActive[cb.Name] != nil)
			if m.isComboDisabled(cb.Name) && m.comboActive[cb.Name] == nil {
				line = disabledStyle.Render(line)
			} else {
				line = comboStyle.Render(line)
			}
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")
	panel := sidebarStyle.Copy().Width(contentWidth).Height(contentHeight)
	if m.focus == focusList {
		panel = panel.BorderForeground(colorAccent)
	} else {
		panel = panel.BorderForeground(colorMuted)
	}
	return panel.Render(content)
}

func (m model) renderOutput(height int) string {
	width := m.width - m.sidebarWidth
	if width < 20 {
		width = 20
	}
	borderWidth, borderHeight := borderSize(outputStyle)
	contentWidth := width - borderWidth
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := height - borderHeight
	if contentHeight < 1 {
		contentHeight = 1
	}
	innerWidth := contentWidth - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	header := "Output"
	status := ""
	if entry := m.selectedEntry(); entry != nil {
		label := entry.Label
		if entry.ParentTask != "" {
			label = fmt.Sprintf("%s > %s", entry.ParentTask, entry.Label)
		}
		header = fmt.Sprintf("Output: %s", label)
		_, status = m.entryStatus(*entry)
		if status != "" {
			header = fmt.Sprintf("%s — %s", header, status)
		}
	}

	statusLine := m.statusBarLine(m.selectedEntry())
	statusText := statusBarStyle.Copy().Width(contentWidth).Render(fitWidth(statusLine, contentWidth))
	statusSpacer := statusBarStyle.Copy().Width(contentWidth).Render(strings.Repeat(" ", contentWidth))
	headerLine := outputContentStyle.Render(fitWidth(header, innerWidth))
	viewportLine := outputContentStyle.Render(m.renderViewport())
	outputLines := []string{
		statusText,
		statusSpacer,
		headerLine,
		viewportLine,
	}
	content := strings.Join(outputLines, "\n")

	panel := outputStyle.Copy().Width(contentWidth).Height(contentHeight)
	if m.focus == focusOutput {
		panel = panel.BorderForeground(colorAccent)
	} else {
		panel = panel.BorderForeground(colorMuted)
	}
	return panel.Render(content)
}

func (m model) renderViewport() string {
	view := m.viewport.View()
	if !m.mouseSelecting {
		return view
	}

	start, end := normalizeSelection(m.selection.Start, m.selection.End)
	if start.Line == end.Line && start.Col == end.Col {
		return view
	}

	lines := strings.Split(view, "\n")
	for i := range lines {
		if i < start.Line || i > end.Line {
			continue
		}

		left := 0
		right := m.viewport.Width
		if i == start.Line {
			left = start.Col
		}
		if i == end.Line {
			right = end.Col + 1
		}

		left = clampInt(left, 0, m.viewport.Width)
		right = clampInt(right, 0, m.viewport.Width)
		if right <= left {
			continue
		}

		lines[i] = applySelectionToLine(lines[i], left, right)
	}

	return strings.Join(lines, "\n")
}

func (m model) renderHelp() string {
	help := "enter: run  ·  ↑/↓: select  ·  ←/→: collapse/expand  ·  ctrl+h/l: focus  ·  ctrl+k/x: kill  ·  ctrl+r: restart  ·  ctrl+z: bg  ·  ctrl+q: quit  ·  ?: help  ·  hotkeys: run"
	return helpStyle.Width(m.width).Render(help)
}

func (m model) renderCheatsheet() string {
	type cheatRow struct {
		key  string
		desc string
	}

	rows := []cheatRow{
		{"↑/k", "Move up"},
		{"↓/j", "Move down"},
		{"←/h", "Collapse group"},
		{"→/l", "Expand group"},
		{"g", "Scroll to top (output)"},
		{"G", "Scroll to bottom (output)"},
		{"enter", "Run selected task/step"},
		{"tab", "Toggle focus list/output"},
		{"ctrl+h", "Focus list"},
		{"ctrl+l", "Focus output"},
		{"ctrl+k or ctrl+x", "Kill selected"},
		{"ctrl+r", "Restart selected task"},
		{"ctrl+z", "Suspend (background)"},
		{"ctrl+q or ctrl+c", "Quit"},
		{"task key", "Run task by hotkey"},
		{"combo key", "Run combo by hotkey"},
		{"q/esc", "Focus list + jump to bottom"},
		{"?", "Close help"},
	}

	keyWidth := 0
	for _, row := range rows {
		if w := ansi.StringWidth(row.key); w > keyWidth {
			keyWidth = w
		}
	}

	lines := []string{modalTitleStyle.Render("Hotkeys"), ""}
	for _, row := range rows {
		keyText := modalKeyStyle.Render(padRight(row.key, keyWidth))
		lines = append(lines, fmt.Sprintf("%s  %s", keyText, row.desc))
	}
	lines = append(lines, "", modalHintStyle.Render("Press ? or Esc to close"))

	body := strings.Join(lines, "\n")
	modal := modalStyle.Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m model) statusBarLine(entry *entry) string {
	if entry == nil {
		return "idle"
	}
	if entry.Kind == entryStep {
		return m.stepStatusLine(entry)
	}
	return m.taskStatusLine(entry)
}

func (m model) stepStatusLine(entry *entry) string {
	step := m.stepByID[entry.Target]
	if step == nil {
		return "idle"
	}
	switch step.Status {
	case StatusRunning:
		return "running"
	case StatusSuccess:
		return "all good"
	case StatusFailed:
		if line := lastLine(step.Output); line != "" {
			return fmt.Sprintf("%s failed: %s", step.Label, line)
		}
		return fmt.Sprintf("%s failed", step.Label)
	case StatusCanceled:
		return fmt.Sprintf("%s canceled", step.Label)
	default:
		return "idle"
	}
}

func (m model) taskStatusLine(entry *entry) string {
	task := m.taskByName[entry.Target]
	if task == nil {
		return "idle"
	}
	if task.Running {
		if total, done := m.taskStepProgress(task); total > 0 {
			return fmt.Sprintf("running (%d/%d)", done, total)
		}
		return "running"
	}
	switch task.Status {
	case StatusSuccess:
		return "all good"
	case StatusFailed:
		if label, line := m.failedStepSummary(task); label != "" {
			return fmt.Sprintf("%s failed: %s", label, line)
		}
		return "failed"
	case StatusCanceled:
		return "canceled"
	default:
		return "idle"
	}
}

func (m model) taskStepProgress(task *Task) (int, int) {
	total := len(task.Steps)
	if total == 0 {
		return 0, 0
	}
	done := 0
	for _, step := range task.Steps {
		run := task.StepRuns[step.ID]
		if run == nil || run.RunSeq != task.RunSeq {
			continue
		}
		if run.Status != StatusIdle && run.Status != StatusRunning {
			done++
		}
	}
	return total, done
}

func (m model) failedStepSummary(task *Task) (string, string) {
	for _, step := range task.Steps {
		run := task.StepRuns[step.ID]
		if run == nil || run.RunSeq != task.RunSeq {
			continue
		}
		if run.Status == StatusFailed {
			line := lastLine(run.Output)
			if step.Kind == StepTask {
				if child := m.taskByName[step.TaskName]; child != nil {
					line = lastLine(child.Output)
				}
			}
			return run.Label, line
		}
		if run.Status == StatusCanceled {
			return run.Label, "canceled"
		}
	}
	return "", ""
}

func lastLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

func fitView(view string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
		if pad := width - ansi.StringWidth(lines[i]); pad > 0 {
			lines[i] = lines[i] + strings.Repeat(" ", pad)
		}
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	if len(lines) < height {
		padLine := strings.Repeat(" ", width)
		for len(lines) < height {
			lines = append(lines, padLine)
		}
	}
	return strings.Join(lines, "\n")
}

func fitWidth(line string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(line, width, "")
}

func borderSize(style lipgloss.Style) (int, int) {
	frameWidth, frameHeight := style.GetFrameSize()
	padTop, padRight, padBottom, padLeft := style.GetPadding()
	borderWidth := frameWidth - padLeft - padRight
	borderHeight := frameHeight - padTop - padBottom
	if borderWidth < 0 {
		borderWidth = 0
	}
	if borderHeight < 0 {
		borderHeight = 0
	}
	return borderWidth, borderHeight
}

func padRight(text string, width int) string {
	if width <= 0 {
		return text
	}
	pad := width - ansi.StringWidth(text)
	if pad <= 0 {
		return text
	}
	return text + strings.Repeat(" ", pad)
}

func fillWidth(line string, width int) string {
	if width <= 0 {
		return ""
	}
	truncated := ansi.Truncate(line, width, "")
	pad := width - ansi.StringWidth(truncated)
	if pad > 0 {
		truncated = truncated + strings.Repeat(" ", pad)
	}
	return truncated
}

func overlayView(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	out := make([]string, len(baseLines))
	for i := range baseLines {
		if i >= len(overlayLines) {
			out[i] = baseLines[i]
			continue
		}
		line := overlayLines[i]
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			out[i] = baseLines[i]
			continue
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
}

func taskDisplayName(def TaskDef) string {
	label := def.Name
	if label == "" {
		label = "task"
	}
	keyLabel := "   "
	if def.Key != "" {
		keyLabel = fmt.Sprintf("[%s]", def.Key)
	}
	return fmt.Sprintf("%s %s", keyLabel, label)
}

func renderComboLine(cb ComboDef, running bool) string {
	base := fmt.Sprintf("[%s] %s (%s)", cb.Key, cb.Name, cb.Mode)
	if !running {
		return base
	}
	return fmt.Sprintf("%s  %s", base, statusStyle(StatusRunning).Render(statusIconRunning))
}

func taskStatusText(task *Task) string {
	if task.Running && task.Def.Persistent {
		return statusIconPersistent
	}
	return statusLabel(task.Status, task.ExitCode)
}

const (
	statusIconRunning    = ""
	statusIconPersistent = ""
	statusIconSuccess    = ""
	statusIconFailed     = ""
	statusIconCanceled   = ""
)

func statusLabel(status TaskStatus, exitCode int) string {
	switch status {
	case StatusRunning:
		return statusIconRunning
	case StatusSuccess:
		return statusIconSuccess
	case StatusFailed:
		if exitCode != 0 {
			return fmt.Sprintf("%s %d", statusIconFailed, exitCode)
		}
		return statusIconFailed
	case StatusCanceled:
		return statusIconCanceled
	default:
		return ""
	}
}

func normalizeSelection(a, b selectionPos) (selectionPos, selectionPos) {
	if b.Line < a.Line || (b.Line == a.Line && b.Col < a.Col) {
		return b, a
	}
	return a, b
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func cutPlain(line string, left, right int) string {
	if right <= left {
		return ""
	}
	return ansi.Strip(ansi.Cut(line, left, right))
}

func trimRightSpaces(line string) string {
	return strings.TrimRight(line, " ")
}

func applySelectionToLine(line string, left, right int) string {
	if right <= left {
		return line
	}
	if left == 0 && right >= ansi.StringWidth(line) {
		return outputSelectionStyle.Render(ansi.Strip(line))
	}

	prefix := ansi.Cut(line, 0, left)
	middle := ansi.Cut(line, left, right)
	suffix := ansi.Cut(line, right, ansi.StringWidth(line))

	return prefix + outputSelectionStyle.Render(ansi.Strip(middle)) + suffix
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if err := copyToSystemClipboard(text); err == nil {
			return nil
		}
		seq := osc52.New(text)
		term := strings.ToLower(os.Getenv("TERM"))
		if strings.Contains(term, "screen") || strings.Contains(term, "tmux") || os.Getenv("TMUX") != "" {
			seq = seq.Screen()
		}
		if os.Getenv("SUITE_OSC52_TMUX") == "1" {
			seq = seq.Tmux()
		}
		fmt.Fprint(os.Stderr, seq.String())
		return nil
	}
}

func copyToSystemClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		path, err := exec.LookPath("pbcopy")
		if err != nil {
			return err
		}
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	default:
		return fmt.Errorf("system clipboard unavailable")
	}
}
