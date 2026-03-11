package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	appFrame         lipgloss.Style
	headerBox        lipgloss.Style
	headerTitle      lipgloss.Style
	statusRunning    lipgloss.Style
	statusStopped    lipgloss.Style
	statusTransition lipgloss.Style
	panel            lipgloss.Style
	panelTitle       lipgloss.Style
	ifaceName        lipgloss.Style
	ifaceUp          lipgloss.Style
	ifaceDown        lipgloss.Style
	muted            lipgloss.Style
	metricLabel      lipgloss.Style
	metricValue      lipgloss.Style
	logLine          lipgloss.Style
	footer           lipgloss.Style
	helpKey          lipgloss.Style
	helpText         lipgloss.Style
	errorText        lipgloss.Style
	warnText         lipgloss.Style
}

func defaultStyles() styles {
	const (
		border = "#2B3D52"
		fg0    = "#E4EEF8"
		fg1    = "#C0D2E2"
		muted  = "#7E97AD"
		accent = "#4CC9F0"
		good   = "#52D273"
		bad    = "#FF6B6B"
		amber  = "#F3A953"
	)

	return styles{
		appFrame: lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg0)),

		headerBox: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color(fg0)).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(border)),

		headerTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(accent)),

		statusRunning: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color(good)).
			Padding(0, 1),

		statusStopped: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color(bad)).
			Padding(0, 1),

		statusTransition: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color(amber)).
			Padding(0, 1),

		panel: lipgloss.NewStyle().
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(border)),

		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(accent)),

		ifaceName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(fg0)),

		ifaceUp: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(good)),

		ifaceDown: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(bad)),

		muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted)),

		metricLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg1)),

		metricValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg0)).
			Bold(true),

		logLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg1)),

		footer: lipgloss.NewStyle().
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(border)),

		helpKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(accent)),

		helpText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg1)),

		errorText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(bad)).
			Bold(true),

		warnText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(amber)).
			Bold(true),
	}
}
