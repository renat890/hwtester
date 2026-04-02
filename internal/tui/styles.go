package tui

import "charm.land/lipgloss/v2"

const (
	headersColor = "#88C0D0"
	defaultColor = "#D8DEE9"
	passColor = "#A3BE8C"
	failColor = "#BF616A"
	errorColor = "#EBCB8B"
	borderColor = "#4C566A"
	accntColor = borderColor
	secondInfoColor = "#616E88"
)

var (
	headStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(2).
				Foreground(lipgloss.Color(headersColor))

	passStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(passColor))
	failStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(failColor))
	errorStyle = lipgloss.NewStyle().
				 Foreground(lipgloss.Color(errorColor))
	
	borderStyle = lipgloss.NewStyle().
				  Foreground(lipgloss.Color(borderColor))
)