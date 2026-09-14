package invoice

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"time"
)

func LoadTemplate(path string) (*template.Template, error) {
	t, err := template.New(filepath.Base(path)).Option("missingkey=error").Funcs(template.FuncMap{
		"money": func(cents int64) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) },
		"date":  func(date time.Time) string { return date.Format("02 Jan 2006") },
	}).ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("parse invoice template: %w", err)
	}
	return t, nil
}

func RenderHTML(t *template.Template, inv Invoice) ([]byte, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, inv); err != nil {
		return nil, fmt.Errorf("render invoice template: %w", err)
	}
	return buf.Bytes(), nil
}
