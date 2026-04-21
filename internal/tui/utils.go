package tui

import "factorytest/internal/hw"

func levelWithStyle(level hw.LogLevel) string {
	switch level {
	case hw.INFO:
		return infoLevelStyle.Render(string(level))
	case hw.WARN:
		return warnLevelStyle.Render(string(level))
	case hw.ERR:
		return errLevelStyle.Render(string(level))
	default:
		return string(level)
	}
}

func statusWithStyle(status hw.Status) string {
	var newStatus string
	switch status {
	case hw.Pass:
		newStatus = passStyle.Render(string(status))
	case hw.Error:
		newStatus = errorStyle.Render(string(status))
	case hw.Fail:
		newStatus = failStyle.Render(string(status))
	case hw.Skip:
		newStatus = accentStyle.Render(string(status))
	}
	return newStatus
}

func textWithStyle(text string, status hw.Status) string {
	var coloredText string
	switch status {
	case hw.Pass:
		coloredText = passStyle.Render(text)
	case hw.Error:
		coloredText = errorStyle.Render(text)
	case hw.Fail:
		coloredText = failStyle.Render(text)
	case hw.Skip:
		coloredText = accentStyle.Render(text)
	}
	return coloredText
}