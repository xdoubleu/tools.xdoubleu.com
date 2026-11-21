package mcptools_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

type noArgs struct{}

func toolsOnServer() []string {
	return []string{
		"factory_list_tasks", "books_get_library", "books_get_shelf",
		"movies_list_backlog", "get_logs", "resolve_sentry_issue",
		"dismiss_security_alert",
	}
}

func callParams(name string) *mcp.CallToolParams {
	return &mcp.CallToolParams{
		Meta:           nil,
		Name:           name,
		Arguments:      nil,
		InputResponses: nil,
		RequestState:   "",
	}
}

func echoTool(
	context.Context, *mcp.CallToolRequest, noArgs,
) (*mcp.CallToolResult, any, error) {
	//nolint:exhaustruct // an empty result is enough
	return &mcp.CallToolResult{}, nil, nil
}

func echoResource(
	_ context.Context, req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	//nolint:exhaustruct // a read sets only the URI
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{URI: req.Params.URI},
	}}, nil
}

// withUser puts user, when set, on the context before the next handler.
func withUser(user *models.User) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			if user != nil {
				ctx = context.WithValue(ctx, constants.UserContextKey, *user)
			}
			return next(ctx, method, req)
		}
	}
}

// connectAs serves toolsOnServer plus a books and a learningpaths resource to a
// client whose calls carry the given user (none when nil).
func connectAs(t *testing.T, user *models.User) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	for _, name := range toolsOnServer() {
		//nolint:exhaustruct // name and description are all a tool needs
		mcp.AddTool(srv, &mcp.Tool{Name: name, Description: name}, echoTool)
	}
	for _, uri := range []string{"books://guide", "learningpaths://guide"} {
		//nolint:exhaustruct // name and URI are all a resource needs
		srv.AddResource(&mcp.Resource{Name: uri, URI: uri}, echoResource)
	}
	// withUser is added first, so it runs first and the filter sees the user.
	srv.AddReceivingMiddleware(withUser(user), mcptools.AgentGrants())

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func toolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := []string{}
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func agent(apps ...string) *models.User {
	u := user(models.RoleAgent)
	u.AppAccess = apps
	return u
}

func user(role models.Role) *models.User {
	return &models.User{
		ID: "u", Email: "u@example.com", Role: role,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false, DisplayName: "",
	}
}

// An agent reaches exactly the apps it was granted, whichever it holds: the
// walk covers no grants and each single grant, and never the admin tools.
func TestAgentReachesOnlyTheAppsItHolds(t *testing.T) {
	ctx := context.Background()
	for grant, want := range map[string][]string{
		"":          {},
		"factory":   {"factory_list_tasks"},
		"books":     {"books_get_library", "books_get_shelf"},
		"movies":    {"movies_list_backlog"},
		"trains":    {},
		"learning":  {},
		"get_logs":  {},
		"admin":     {},
		"notanapp_": {},
	} {
		apps := []string{}
		if grant != "" {
			apps = append(apps, grant)
		}
		session := connectAs(t, agent(apps...))

		assert.ElementsMatch(t, want, toolNames(t, session), grant)
		for _, name := range append([]string{"nonexistent"}, toolsOnServer()...) {
			_, err := session.CallTool(ctx, callParams(name))
			if contains(want, name) {
				assert.NoError(t, err, grant+" "+name)
				continue
			}
			require.Error(t, err, grant+" "+name)
			assert.Contains(t, err.Error(), "unknown tool", grant+" "+name)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestAgentResourcesFollowItsGrants(t *testing.T) {
	ctx := context.Background()
	session := connectAs(t, agent("books"))

	list, err := session.ListResources(ctx, nil)
	require.NoError(t, err)
	require.Len(t, list.Resources, 1)
	assert.Equal(t, "books://guide", list.Resources[0].URI)

	//nolint:exhaustruct // the URI is all a read needs
	_, err = session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "books://guide"})
	assert.NoError(t, err)
	//nolint:exhaustruct // the URI is all a read needs
	_, err = session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "learningpaths://guide",
	})
	assert.Error(t, err)
}

func TestAgentCannotUsePrompts(t *testing.T) {
	session := connectAs(t, agent("books", "factory"))

	_, err := session.ListPrompts(context.Background(), nil)
	assert.Error(t, err)
}

func TestCallersWithoutTheGrantDoNotSeeTheBoardTools(t *testing.T) {
	for name, u := range map[string]*models.User{
		"user":    user(models.RoleUser),
		"service": user(models.RoleService),
		"nobody":  nil,
	} {
		session := connectAs(t, u)
		assert.ElementsMatch(t,
			[]string{
				"books_get_library", "books_get_shelf", "movies_list_backlog",
				"get_logs", "resolve_sentry_issue", "dismiss_security_alert",
			},
			toolNames(t, session), name)
		_, err := session.CallTool(context.Background(), callParams("books_get_library"))
		assert.NoError(t, err, name)
	}
}

func TestAdminsAndGrantedPeopleSeeTheBoardTools(t *testing.T) {
	granted := user(models.RoleUser)
	granted.AppAccess = []string{"factory"}
	for name, u := range map[string]*models.User{
		"admin":   user(models.RoleAdmin),
		"granted": granted,
	} {
		assert.Contains(t, toolNames(t, connectAs(t, u)), "factory_list_tasks", name)
	}
}

func ctxWith(u *models.User) context.Context {
	ctx := context.Background()
	if u != nil {
		ctx = context.WithValue(ctx, constants.UserContextKey, *u)
	}
	return ctx
}

func TestRequireAgentAdmitsOnlyAnAgentAccount(t *testing.T) {
	assert.NoError(t, mcptools.RequireAgent(ctxWith(user(models.RoleAgent))))
	for name, u := range map[string]*models.User{
		"admin":   user(models.RoleAdmin),
		"user":    user(models.RoleUser),
		"service": user(models.RoleService),
		"nobody":  nil,
	} {
		err := mcptools.RequireAgent(ctxWith(u))
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "agent accounts only", name)
	}
}

func TestWriteNeedsTheWriteLevelForAnAgentAndAnyAccessForAPerson(t *testing.T) {
	granted := func(role models.Role, access, write []string) *models.User {
		u := user(role)
		u.AppAccess, u.WriteAccess = access, write
		return u
	}
	for name, tc := range map[string]struct {
		user      *models.User
		read, wrt bool
	}{
		"admin":        {user(models.RoleAdmin), true, true},
		"person":       {granted(models.RoleUser, []string{"books"}, nil), true, true},
		"person other": {granted(models.RoleUser, []string{"feeds"}, nil), false, false},
		"agent read":   {granted(models.RoleAgent, []string{"books"}, nil), true, false},
		"agent write": {
			granted(models.RoleAgent, []string{"books"}, []string{"books"}), true, true,
		},
		"agent no grant": {user(models.RoleAgent), false, false},
		"nobody":         {nil, false, false},
	} {
		assert.Equal(t, tc.read,
			mcptools.RequireAppAccess(ctxWith(tc.user), "books") == nil, name)
		assert.Equal(t, tc.wrt,
			mcptools.RequireAppWrite(ctxWith(tc.user), "books") == nil, name)
	}
}

// The observability grant opens the read tools and resolve_sentry_issue, and
// nothing else: not the admin-only dismissal, not another app's tools.
func TestObservabilityGrantOpensOnlyItsTools(t *testing.T) {
	session := connectAs(t, agent("factory", mcptools.ObservabilityGrant))

	assert.ElementsMatch(t,
		[]string{"factory_list_tasks", "get_logs", "resolve_sentry_issue"},
		toolNames(t, session))
	_, err := session.CallTool(context.Background(), callParams("resolve_sentry_issue"))
	require.NoError(t, err)
	_, err = session.CallTool(context.Background(), callParams("dismiss_security_alert"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestWithoutTheObservabilityGrantAnAgentSeesNoObservabilityTools(t *testing.T) {
	session := connectAs(t, agent("factory"))

	assert.Equal(t, []string{"factory_list_tasks"}, toolNames(t, session))
}

func TestCanObserveByRoleAndGrant(t *testing.T) {
	withRole := func(u *models.User) context.Context {
		if u == nil {
			return context.Background()
		}
		return context.WithValue(context.Background(), constants.UserContextKey, *u)
	}
	assert.True(t, mcptools.CanObserve(withRole(user(models.RoleAdmin)), "get_logs"))
	assert.True(t, mcptools.CanObserve(withRole(user(models.RoleService)), "get_logs"))
	assert.False(t, mcptools.CanObserve(withRole(user(models.RoleUser)), "get_logs"))
	assert.False(t, mcptools.CanObserve(withRole(nil), "get_logs"))
	assert.False(t, mcptools.CanObserve(withRole(agent("factory")), "get_logs"))
	granted := agent(mcptools.ObservabilityGrant)
	assert.True(t, mcptools.CanObserve(withRole(granted), "prom_query"))
	assert.False(t, mcptools.CanObserve(withRole(granted), "dismiss_security_alert"))
}

// A per-user app's tools and resources run as the person the agent acts for,
// while factory_ and admin tools keep the agent's own identity (adr-0057).
func TestAgentActsAsItsPrincipalOnPerUserApps(t *testing.T) {
	ctx := context.Background()
	seen := map[string]string{}
	record := func(c context.Context, name string) {
		u := contexttools.GetValue[models.User](c, constants.UserContextKey)
		seen[name] = u.ID + "/" + string(u.Role)
	}

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	for _, name := range []string{"learningpaths_list_paths", "factory_list_tasks"} {
		//nolint:exhaustruct // name and description are all a tool needs
		mcp.AddTool(srv, &mcp.Tool{Name: name, Description: name},
			func(c context.Context, _ *mcp.CallToolRequest, _ noArgs) (
				*mcp.CallToolResult, any, error,
			) {
				record(c, name)
				//nolint:exhaustruct // an empty result is enough
				return &mcp.CallToolResult{}, nil, nil
			})
	}
	//nolint:exhaustruct // name and URI are all a resource needs
	srv.AddResource(&mcp.Resource{Name: "g", URI: "learningpaths://guide"},
		func(c context.Context, req *mcp.ReadResourceRequest) (
			*mcp.ReadResourceResult, error,
		) {
			record(c, "resource")
			return echoResource(c, req)
		})
	acting := agent("learningpaths", "factory")
	acting.PrincipalID = "owner"
	srv.AddReceivingMiddleware(withUser(acting), mcptools.AgentGrants())

	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })
	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	for _, name := range []string{"learningpaths_list_paths", "factory_list_tasks"} {
		_, err = session.CallTool(ctx, callParams(name))
		require.NoError(t, err)
	}
	_, err = session.ReadResource(ctx, &mcp.ReadResourceParams{
		Meta: nil, URI: "learningpaths://guide", InputResponses: nil, RequestState: "",
	})
	require.NoError(t, err)

	assert.Equal(t, "owner/agent", seen["learningpaths_list_paths"])
	assert.Equal(t, "owner/agent", seen["resource"])
	assert.Equal(t, "u/agent", seen["factory_list_tasks"])
}
