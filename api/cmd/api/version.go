package main

import (
	"net/http"

	httptools "tools.xdoubleu.com/internal/communication/httptools"
)

func (app *Application) versionHandler(w http.ResponseWriter, r *http.Request) {
	if err := httptools.WriteJSON(
		w,
		http.StatusOK,
		map[string]string{"release": app.config.Release},
		nil,
	); err != nil {
		httptools.HandleError(w, r, err)
	}
}
