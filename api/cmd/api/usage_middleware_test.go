package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsageLabels(t *testing.T) {
	appNames := map[string]bool{"books": true, "games": true, "todos": true}

	tests := []struct {
		name         string
		method       string
		path         string
		wantApp      string
		wantEndpoint string
		wantOK       bool
	}{
		{
			name:         "connectrpc path",
			method:       http.MethodPost,
			path:         "/todos/todos.v1.TaskService/ListTasks",
			wantApp:      "todos",
			wantEndpoint: "TaskService/ListTasks",
			wantOK:       true,
		},
		{
			name:         "global connectrpc path",
			method:       http.MethodPost,
			path:         "/admin.v1.AdminService/ListUsers",
			wantApp:      "global",
			wantEndpoint: "AdminService/ListUsers",
			wantOK:       true,
		},
		{
			name:         "kobo token masked",
			method:       http.MethodGet,
			path:         "/books/kobo/9f8b2c1d4e5a6b7c8d9e0f1a2b3c4d5e/sync",
			wantApp:      "books",
			wantEndpoint: "kobo",
			wantOK:       true,
		},
		{
			name:         "uuid segment masked",
			method:       http.MethodGet,
			path:         "/games/4001e9cf-3fbe-4b09-863f-bd1654cfbf76",
			wantApp:      "games",
			wantEndpoint: ":id",
			wantOK:       true,
		},
		{
			name:         "app root",
			method:       http.MethodGet,
			path:         "/books/",
			wantApp:      "books",
			wantEndpoint: "root",
			wantOK:       true,
		},
		{
			name:         "health skipped",
			method:       http.MethodGet,
			path:         "/health",
			wantApp:      "",
			wantEndpoint: "",
			wantOK:       false,
		},
		{
			name:         "version skipped",
			method:       http.MethodGet,
			path:         "/api/version",
			wantApp:      "",
			wantEndpoint: "",
			wantOK:       false,
		},
		{
			name:         "preflight skipped",
			method:       http.MethodOptions,
			path:         "/books/todos.v1.TaskService/ListTasks",
			wantApp:      "",
			wantEndpoint: "",
			wantOK:       false,
		},
		{
			name:         "empty path skipped",
			method:       http.MethodGet,
			path:         "/",
			wantApp:      "",
			wantEndpoint: "",
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			appName, endpoint, ok := usageLabels(req, appNames)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantApp, appName)
				assert.Equal(t, tt.wantEndpoint, endpoint)
			}
		})
	}
}
