package api

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed web/templates/* web/static/*
var webContent embed.FS

// getFileSystem gets the embedded static files as an http.FileSystem
func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(webContent, "web/static")
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}

// loadEmbeddedTemplates loads templates from the embedded filesystem
func loadEmbeddedTemplates() (*template.Template, error) {
	return template.ParseFS(webContent,
		"web/templates/base.html",
		"web/templates/home.html",
		"web/templates/dns.html",
		"web/templates/certs.html",
	)
}
