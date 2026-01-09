package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type CommandList []string

func (c *CommandList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			return nil
		}
		cmd := strings.TrimSpace(value.Value)
		if cmd == "" {
			*c = nil
			return nil
		}
		*c = CommandList{cmd}
		return nil
	case yaml.SequenceNode:
		cmds := make(CommandList, 0, len(value.Content))
		for _, node := range value.Content {
			if node.Kind != yaml.ScalarNode {
				return fmt.Errorf("commands must be strings")
			}
			cmds = append(cmds, strings.TrimSpace(node.Value))
		}
		*c = cmds
		return nil
	case 0:
		return nil
	default:
		return fmt.Errorf("commands must be a string or list")
	}
}
