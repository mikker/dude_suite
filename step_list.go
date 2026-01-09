package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type StepKind int

const (
	StepAuto StepKind = iota
	StepCommand
	StepTask
)

type Step struct {
	Value string
	Name  string
	Kind  StepKind
}

type StepList []Step

func (s *StepList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			return nil
		}
		val := strings.TrimSpace(value.Value)
		if val == "" {
			*s = nil
			return nil
		}
		*s = StepList{{Value: val, Kind: StepAuto}}
		return nil
	case yaml.SequenceNode:
		steps := make(StepList, 0, len(value.Content))
		for _, node := range value.Content {
			step, err := parseStepNode(node)
			if err != nil {
				return err
			}
			steps = append(steps, step)
		}
		*s = steps
		return nil
	case 0:
		return nil
	default:
		return fmt.Errorf("steps must be a string or list")
	}
}

func parseStepNode(node *yaml.Node) (Step, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return Step{Value: strings.TrimSpace(node.Value), Kind: StepAuto}, nil
	case yaml.MappingNode:
		var (
			name    string
			cmd     string
			task    string
			cmdSet  bool
			taskSet bool
		)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Kind != yaml.ScalarNode {
				continue
			}
			k := strings.TrimSpace(key.Value)
			switch k {
			case "name":
				if val.Kind != yaml.ScalarNode {
					return Step{}, fmt.Errorf("step name must be a string")
				}
				name = strings.TrimSpace(val.Value)
			case "cmd":
				if val.Kind != yaml.ScalarNode {
					return Step{}, fmt.Errorf("step cmd must be a string")
				}
				cmd = strings.TrimSpace(val.Value)
				cmdSet = true
			case "task":
				if val.Kind != yaml.ScalarNode {
					return Step{}, fmt.Errorf("step task must be a string")
				}
				task = strings.TrimSpace(val.Value)
				taskSet = true
			}
		}
		if cmdSet && taskSet {
			return Step{}, fmt.Errorf("step cannot define both cmd and task")
		}
		if cmdSet {
			return Step{Value: cmd, Name: name, Kind: StepCommand}, nil
		}
		if taskSet {
			return Step{Value: task, Name: name, Kind: StepTask}, nil
		}
		return Step{}, fmt.Errorf("step must be a string, {cmd: ...}, or {task: ...}")
	default:
		return Step{}, fmt.Errorf("step must be a string or map")
	}
}
