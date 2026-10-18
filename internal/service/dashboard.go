package service

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var assets embed.FS

func dashboard() http.Handler {
	files, _ := fs.Sub(assets, "web")
	return http.FileServer(http.FS(files))
}
