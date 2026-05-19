package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed static/images/*
var images embed.FS

func (app *Application) imageRoutes(prefix string, mux *http.ServeMux) {
	sub, err := fs.Sub(images, "static/images")
	if err != nil {
		panic(err)
	}

	mux.Handle(fmt.Sprintf("GET /%s/", prefix),
		http.StripPrefix(fmt.Sprintf("/%s/", prefix),
			http.FileServer(http.FS(sub)),
		),
	)
}
