package report

import (
	"embed"
	"encoding/json"
	"errors"
	"factorytest/internal/hw"
	"fmt"
	"html/template"
	"io"
	"time"
)

var (
	ErrWriteJSON = errors.New("не смог записать в JSON")
	ErrWriteHTML = errors.New("не смог записать в файл HTML")
)

//go:embed template/*.html
var templateFS embed.FS

func Generate(meta Meta, results []hw.TestResult) Report {
	final := hw.Pass

	for _, result := range results {
		if result.Status == hw.Fail || result.Status == hw.Error {
			final = hw.Fail
			break
		}
	}

	return Report{
		Meta:        meta,
		Results:     results,
		FinalResult: final,
	}
}

func WriteJSON(w io.Writer, r Report) error {
	const prefix = ""
	const indent = "  "
	body, err := json.MarshalIndent(r, prefix, indent)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteJSON, err)
	}
	_, err = w.Write(body)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteJSON, err)
	}
	return nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatDate": func (t time.Time, a string) string {
			return t.Format(a)
		},
		"add": func(a, b int) int { return a + b },
	}
}

func WriteHTML(w io.Writer, r Report) error {
	const templateName = "report.html"
	t, err := template.New("report").Funcs(templateFuncs()).ParseFS(templateFS, "template/*.html")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteHTML, err)
	}
	if err := t.ExecuteTemplate(w, templateName, r); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteHTML, err)
	}
	return nil
}