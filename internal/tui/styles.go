package tui

import "charm.land/lipgloss/v2"

const (
	headerColor = "#88C0D0"
	defaultColor = "#D8DEE9"
	passColor = "#A3BE8C"
	failColor = "#BF616A"
	errorColor = "#EBCB8B"
	borderColor = "#4C566A"
	accentColor = borderColor
	secondInfoColor = "#616E88"
)

var (
	headStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(2).
				Foreground(lipgloss.Color(headerColor))

	head2Style = lipgloss.NewStyle().
				 Foreground(lipgloss.Color(headerColor))

	passStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(passColor))
	failStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(failColor))
	errorStyle = lipgloss.NewStyle().
				 Foreground(lipgloss.Color(errorColor))
	
	borderStyle = lipgloss.NewStyle().
				  BorderForeground(lipgloss.Color(borderColor)).
				  BorderStyle(lipgloss.NormalBorder()).
				  Padding(0, 1)
	
	spinnerStyle = lipgloss.NewStyle().
				   Foreground(lipgloss.Color(headerColor))

	infoLevelStyle = lipgloss.NewStyle().
					 Foreground(lipgloss.Color(passColor))
	warnLevelStyle = lipgloss.NewStyle().
					 Foreground(lipgloss.Color(failColor))
    errLevelStyle = lipgloss.NewStyle().
					 Foreground(lipgloss.Color(errorColor))
				
)