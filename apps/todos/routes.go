package todos

import "net/http"

func (a *Todos) Routes(prefix string, mux *http.ServeMux) {
	auth := a.services.Auth.AppAccess

	// Static paths must come before /{id} to avoid wildcard matching.
	mux.HandleFunc("GET /"+prefix+"/{$}",
		auth(prefix, a.handle(a.listTasksHandler)))
	mux.HandleFunc("POST /"+prefix+"/{$}",
		auth(prefix, a.handle(a.quickAddHandler)))

	mux.HandleFunc("GET /"+prefix+"/done",
		auth(prefix, a.handle(a.listDoneHandler)))
	mux.HandleFunc("GET /"+prefix+"/archive",
		auth(prefix, a.handle(a.listArchiveHandler)))
	mux.HandleFunc("GET /"+prefix+"/search",
		auth(prefix, a.handle(a.searchHandler)))

	mux.HandleFunc("POST /"+prefix+"/reorder",
		auth(prefix, a.handle(a.reorderHandler)))

	mux.HandleFunc("GET /"+prefix+"/new",
		auth(prefix, a.handle(a.newTaskFormHandler)))
	mux.HandleFunc("POST /"+prefix+"/new",
		auth(prefix, a.handle(a.createTaskHandler)))

	mux.HandleFunc("GET /"+prefix+"/settings",
		auth(prefix, a.handle(a.settingsHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/archive",
		auth(prefix, a.handle(a.updateArchiveHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/labels",
		auth(prefix, a.handle(a.addLabelHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/labels/{category}/{value}/delete",
		auth(prefix, a.handle(a.removeLabelHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/url-patterns",
		auth(prefix, a.handle(a.addURLPatternHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/url-patterns/{id}/delete",
		auth(prefix, a.handle(a.removeURLPatternHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/sections",
		auth(prefix, a.handle(a.addSectionHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/sections/{id}/delete",
		auth(prefix, a.handle(a.removeSectionHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/policies",
		auth(prefix, a.handle(a.addPolicyHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/policies/{id}/delete",
		auth(prefix, a.handle(a.removePolicyHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/workspaces",
		auth(prefix, a.handle(a.addWorkspaceHandler)))
	mux.HandleFunc("POST /"+prefix+"/settings/workspaces/{id}/delete",
		auth(prefix, a.handle(a.deleteWorkspaceHandler)))

	mux.HandleFunc("POST /"+prefix+"/mode",
		auth(prefix, a.handle(a.setModeHandler)))

	// Parameterised paths last.
	mux.HandleFunc("GET /"+prefix+"/{id}",
		auth(prefix, a.handle(a.viewTaskHandler)))
	mux.HandleFunc("GET /"+prefix+"/{id}/edit",
		auth(prefix, a.handle(a.editTaskFormHandler)))
	mux.HandleFunc("POST /"+prefix+"/{id}/edit",
		auth(prefix, a.handle(a.updateTaskHandler)))
	mux.HandleFunc("POST /"+prefix+"/{id}/complete",
		auth(prefix, a.handle(a.completeTaskHandler)))
	mux.HandleFunc("POST /"+prefix+"/{id}/reopen",
		auth(prefix, a.handle(a.reopenTaskHandler)))
	mux.HandleFunc("POST /"+prefix+"/{id}/delete",
		auth(prefix, a.handle(a.deleteTaskHandler)))

	mux.HandleFunc("POST /"+prefix+"/{id}/subtasks",
		auth(prefix, a.handle(a.addSubtaskHandler)))
	mux.HandleFunc("POST /"+prefix+"/{id}/subtasks/{sid}/toggle",
		auth(prefix, a.handle(a.toggleSubtaskHandler)))
	mux.HandleFunc("POST /"+prefix+"/{id}/subtasks/{sid}/delete",
		auth(prefix, a.handle(a.deleteSubtaskHandler)))
}
