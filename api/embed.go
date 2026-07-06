package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/ElementalMP4/loggerino/log"
)

//go:embed web/*
var webContent embed.FS

func subFS(dir string) http.FileSystem {
	fsys, err := fs.Sub(webContent, "web/"+dir)
	if err != nil {
		log.Fatalf("embed", "embedded fs error: %v", err)
	}
	return http.FS(fsys)
}

func getCSSFileSystem() http.FileSystem  { return subFS("css") }
func getJSFileSystem() http.FileSystem   { return subFS("js") }
func getHTMLFileSystem() http.FileSystem { return subFS("html") }
