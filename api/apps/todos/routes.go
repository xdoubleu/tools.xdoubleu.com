package todos

import (
	"net/http"

	"tools.xdoubleu.com/gen/todos/v1/todosv1connect"
)

func (a *Todos) Routes(prefix string, mux *http.ServeMux) {
	auth := a.services.Auth.AppAccess

	taskPath, taskHandler := todosv1connect.NewTaskServiceHandler(
		&taskConnectHandler{app: a},
	)
	mux.Handle("POST "+taskPath, auth(prefix, taskHandler.ServeHTTP))

	subtaskPath, subtaskHandler := todosv1connect.NewSubtaskServiceHandler(
		&subtaskConnectHandler{app: a},
	)
	mux.Handle("POST "+subtaskPath, auth(prefix, subtaskHandler.ServeHTTP))

	settingsPath, settingsHandler := todosv1connect.NewSettingsServiceHandler(
		&settingsConnectHandler{app: a},
	)
	mux.Handle("POST "+settingsPath, auth(prefix, settingsHandler.ServeHTTP))
}
