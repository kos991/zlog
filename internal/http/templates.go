package http

import (
	"html/template"
	"os"
	"path/filepath"
)

var templateDir = "web/templates"

func loadTemplate(name string) *template.Template {
	dir := templateDir
	if envDir := os.Getenv("ZLOG_TEMPLATE_DIR"); envDir != "" {
		dir = envDir
	}
	path := filepath.Join(dir, name+".html")
	tmpl, err := template.ParseFiles(
		filepath.Join(dir, "base.html"),
		path,
	)
	if err != nil {
		return nil
	}
	return tmpl
}
