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

	// Login page renders standalone (no navbar)
	if name == "login" {
		path := filepath.Join(dir, name+".html")
		tmpl, err := template.ParseFiles(path)
		if err != nil {
			return nil
		}
		return tmpl
	}

	// All other pages use base.html layout
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
