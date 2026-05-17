package todos

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const (
	todosRoot         = "/todos/"
	subtaskSourceView = "view"
	subtaskSourceList = "list"
	tabOpen           = "open"
)

func safeBackRedirect(back string) string {
	if back == "" {
		return todosRoot
	}

	normalized := strings.ReplaceAll(back, "\\", "/")
	target, err := url.Parse(normalized)
	if err != nil {
		return todosRoot
	}

	// Allow only local redirects (no host/scheme).
	if target.Hostname() != "" || target.IsAbs() {
		return todosRoot
	}

	// Require an absolute local path for predictable behavior.
	if !strings.HasPrefix(target.Path, "/") {
		return todosRoot
	}

	return target.String()
}

func safeLocalRedirectTarget(rawBack string) string {
	if rawBack == "" {
		return todosRoot
	}

	normalized := strings.ReplaceAll(rawBack, "\\", "/")
	target, err := url.Parse(normalized)
	if err != nil {
		return todosRoot
	}

	if target.Hostname() != "" || target.Scheme != "" {
		return todosRoot
	}

	if !strings.HasPrefix(target.Path, "/") {
		return todosRoot
	}

	return target.String()
}

func isHXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

type workspaceCtx struct {
	Settings   *models.UserSettings
	Workspaces []models.Workspace
}

func (a *Todos) loadWorkspaceCtx(
	ctx context.Context,
	userID string,
) (*workspaceCtx, error) {
	settings, err := a.services.Settings.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	workspaces, err := a.services.Workspaces.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &workspaceCtx{Settings: settings, Workspaces: workspaces}, nil
}

type SearchResults struct {
	Open     []models.Task
	Done     []models.Task
	Archived []models.Task
}

func currentUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

// workspaceQuery returns the URL query segment that encodes the active workspace,
// e.g. "w=550e8400-…" or "w=private".
func workspaceQuery(wsID *uuid.UUID) string {
	if wsID == nil {
		return "w=private"
	}
	return "w=" + wsID.String()
}

func wsIDsEqual(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// applyWorkspaceParam reads ?w= from r, updates DB if it changed, and returns
// true + a redirect URL when the caller should redirect (missing ?w= on a full
// page load). The wsCtx is updated in-place when ?w= is present.
func (a *Todos) applyWorkspaceParam(
	r *http.Request,
	wsCtx *workspaceCtx,
	userID string,
) (string, bool) {
	rawW := r.URL.Query().Get("w")
	if rawW == "" {
		if isHXRequest(r) {
			return "", false // HTMX partial — use DB workspace, no redirect
		}
		target := "/todos/?" + workspaceQuery(wsCtx.Settings.ActiveWorkspaceID)
		if s := r.URL.Query().Get("section"); s != "" {
			target += "&section=" + s
		}
		return target, true
	}
	var newWsID *uuid.UUID
	if rawW != "private" {
		if id, parseErr := uuid.Parse(rawW); parseErr == nil {
			newWsID = &id
		}
	}
	if !wsIDsEqual(wsCtx.Settings.ActiveWorkspaceID, newWsID) {
		_ = a.services.Settings.SetActiveWorkspace(r.Context(), userID, newWsID)
		wsCtx.Settings.ActiveWorkspaceID = newWsID
		wsCtx.Settings.ActiveWorkspace = nil
		if newWsID != nil {
			for i := range wsCtx.Workspaces {
				if wsCtx.Workspaces[i].ID == *newWsID {
					wsCtx.Settings.ActiveWorkspace = &wsCtx.Workspaces[i]
					break
				}
			}
		}
	}
	return "", false
}

func (a *Todos) loadLabelColors(
	ctx context.Context,
	userID string,
	wsID *uuid.UUID,
) map[string]string {
	presets, err := a.services.Settings.GetLabelPresets(ctx, userID, wsID)
	if err != nil {
		return map[string]string{}
	}
	return presets.ColorMap()
}

func parseSectionID(raw string) *uuid.UUID {
	if raw == "" {
		return nil
	}
	if sid, err := uuid.Parse(raw); err == nil {
		return &sid
	}
	return nil
}
