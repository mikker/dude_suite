package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(defaultConfigName); err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "Created %s. Edit it, then re-run suite.\n", defaultConfigName)
		return
	}

	var configPath string
	var themeFlag string
	flag.StringVar(&configPath, "config", defaultConfigName, "path to config file")
	flag.StringVar(&configPath, "c", defaultConfigName, "path to config file (shorthand)")
	flag.StringVar(&themeFlag, "theme", "", "theme override: auto, light, or dark")
	flag.StringVar(&themeFlag, "t", "", "theme override: auto, light, or dark (shorthand)")
	flag.Parse()

	cfg, err := LoadConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && configPath == defaultConfigName {
			if err := offerInit(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "config error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if strings.TrimSpace(themeFlag) != "" {
		cfg.Theme = strings.TrimSpace(themeFlag)
	}
	applyTheme(cfg.Theme)
	m := newModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		os.Exit(1)
	}
}

func applyTheme(theme string) {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "light":
		lipgloss.SetHasDarkBackground(false)
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	default:
		if os.Getenv("TMUX") != "" {
			if dark, ok := detectDarkBackgroundFromEnv(); ok {
				lipgloss.SetHasDarkBackground(dark)
			}
		}
	}
}

func detectDarkBackgroundFromEnv() (bool, bool) {
	value := strings.TrimSpace(os.Getenv("COLORFGBG"))
	if value == "" {
		return false, false
	}
	parts := strings.Split(value, ";")
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" || strings.EqualFold(last, "default") {
		return false, false
	}
	bg, err := strconv.Atoi(last)
	if err != nil {
		return false, false
	}
	return bg <= 6, true
}
