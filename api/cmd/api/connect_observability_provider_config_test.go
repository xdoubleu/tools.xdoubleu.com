package main

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

func TestProtoProviderConfig(t *testing.T) {
	assert.Nil(t, protoProviderConfig(models.OAuthProviderGithub, nil), "unset config")
	assert.Nil(
		t, protoProviderConfig(models.OAuthProviderGithub, []byte(`not json`)),
		"malformed JSON degrades to unset rather than erroring",
	)
	assert.Nil(
		t, protoProviderConfig(models.OAuthProviderGithub, []byte(`{"repo":""}`)),
		"empty repo degrades to unset",
	)
	assert.Nil(
		t, protoProviderConfig(models.OAuthProviderSentry, []byte(`{"org":""}`)),
		"empty org degrades to unset",
	)
	assert.Nil(t, protoProviderConfig("unknown", []byte(`{}`)), "unknown provider")

	cfg := protoProviderConfig(
		models.OAuthProviderSentry, []byte(`{"org":"o","projects":["a","b"]}`),
	)
	require.NotNil(t, cfg)
	assert.Equal(t, "o", cfg.GetSentry().GetOrg())
	assert.Equal(t, []string{"a", "b"}, cfg.GetSentry().GetProjects())
}

func TestGetProviderOptions_Github(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `[{"full_name":"o/a"},{"full_name":"o/b"}]`)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderGithub),
	}
	resp, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"o/a", "o/b"}, resp.Repos)
}

func TestGetProviderOptions_SentryOrgsThenProjects(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `[{"slug":"proj-a"}]`)
	sentryapi.SetBaseURL(srv.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"), configNotConnected(),
	)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider:  string(models.OAuthProviderSentry),
		SentryOrg: "org-a",
	}
	resp, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"proj-a"}, resp.SentryProjects)
	assert.Empty(t, resp.SentryOrgs)
}

func TestGetProviderOptions_SentryOrgs(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `[{"slug":"org-a"}]`)
	sentryapi.SetBaseURL(srv.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"), configNotConnected(),
	)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderSentry),
	}
	resp, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"org-a"}, resp.SentryOrgs)
}

func TestGetProviderOptions_SentryOrgsError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok(""), configNotConnected(),
	)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderSentry),
	}
	_, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestGetProviderOptions_SentryProjectsError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok(""), configNotConnected(),
	)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider:  string(models.OAuthProviderSentry),
		SentryOrg: "org-a",
	}
	_, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestGetProviderOptions_NotConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok(""),
		configNotConnected(),
	)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderGithub),
	}
	_, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestGetProviderOptions_UnknownProvider(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider: "not-a-provider",
	}
	_, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetProviderOptions_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := &observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderGithub),
	}
	_, err := observabilityClient(
		t,
	).GetProviderOptions(withCookies(context.Background(), accessToken), req)
	requirePermissionDenied(t, err)
}

func TestSetProviderConfig_Github(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other token fields unused in test
			AccessToken: "tok",
		},
		testUserID,
		nil,
	))

	req := &observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: "o/r"},
			},
		},
	}
	_, err := observabilityClient(
		t,
	).SetProviderConfig(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)

	_, conn, err := testApp.oauthConnRepo.Get(t.Context(), models.OAuthProviderGithub)
	require.NoError(t, err)
	assert.JSONEq(t, `{"repo":"o/r"}`, string(conn.Config))

	listReq := &observabilityv1.ListOAuthConnectionsRequest{}
	listResp, err := observabilityClient(
		t,
	).ListOAuthConnections(withCookies(context.Background(), accessToken), listReq)
	require.NoError(t, err)
	for _, c := range listResp.Connections {
		if c.Provider == string(models.OAuthProviderGithub) {
			require.NotNil(t, c.Config)
			assert.Equal(t, "o/r", c.Config.GetGithub().GetRepo())
		}
	}
}

func TestSetProviderConfig_Sentry(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderSentry,
		&oauth2.Token{ //nolint:exhaustruct // other token fields unused in test
			AccessToken: "tok",
		},
		testUserID,
		nil,
	))

	req := &observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderSentry),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Sentry{
				Sentry: &observabilityv1.SentryConfig{
					Org: "org-a", Projects: []string{"p1", "p2"},
				},
			},
		},
	}
	_, err := observabilityClient(
		t,
	).SetProviderConfig(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)

	_, conn, err := testApp.oauthConnRepo.Get(t.Context(), models.OAuthProviderSentry)
	require.NoError(t, err)
	assert.JSONEq(t, `{"org":"org-a","projects":["p1","p2"]}`, string(conn.Config))
}

func TestSetProviderConfig_MissingFields(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	req := &observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config:   &observabilityv1.ProviderConfig{},
	}
	_, err := observabilityClient(
		t,
	).SetProviderConfig(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetProviderConfig_UnknownProvider(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	req := &observabilityv1.SetProviderConfigRequest{
		Provider: "not-a-provider",
		Config:   &observabilityv1.ProviderConfig{},
	}
	_, err := observabilityClient(
		t,
	).SetProviderConfig(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetProviderConfig_NotConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	req := &observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: "o/r"},
			},
		},
	}
	_, err := observabilityClient(
		t,
	).SetProviderConfig(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSetProviderConfig_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := &observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: "o/r"},
			},
		},
	}
	_, err := observabilityClient(
		t,
	).SetProviderConfig(withCookies(context.Background(), accessToken), req)
	requirePermissionDenied(t, err)
}
