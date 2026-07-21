package main

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"golang.org/x/oauth2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
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
	assert.Nil(
		t,
		protoProviderConfig(models.OAuthProviderDigitalOcean, []byte(`{"app_id":""}`)),
		"empty app_id degrades to unset",
	)
	assert.Nil(t, protoProviderConfig("unknown", []byte(`{}`)), "unknown provider")

	cfg := protoProviderConfig(
		models.OAuthProviderSentry, []byte(`{"org":"o","projects":["a","b"]}`),
	)
	require.NotNil(t, cfg)
	assert.Equal(t, "o", cfg.GetSentry().GetOrg())
	assert.Equal(t, []string{"a", "b"}, cfg.GetSentry().GetProjects())

	cfg = protoProviderConfig(
		models.OAuthProviderDigitalOcean, []byte(`{"app_id":"app-1"}`),
	)
	require.NotNil(t, cfg)
	assert.Equal(t, "app-1", cfg.GetDigitalocean().GetAppId())
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

	req := connect.NewRequest(&observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetProviderOptions(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"o/a", "o/b"}, resp.Msg.Repos)
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

	req := connect.NewRequest(&observabilityv1.GetProviderOptionsRequest{
		Provider:  string(models.OAuthProviderSentry),
		SentryOrg: "org-a",
	})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetProviderOptions(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"proj-a"}, resp.Msg.SentryProjects)
	assert.Empty(t, resp.Msg.SentryOrgs)
}

func TestGetProviderOptions_DigitalOcean(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(
		t,
		http.StatusOK,
		`{"apps":[{"id":"id-1","spec":{"name":"app-one"}}]}`,
	)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(), stubTok("tok"), configNotConnected(),
	)

	req := connect.NewRequest(&observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderDigitalOcean),
	})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetProviderOptions(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"id-1 — app-one"}, resp.Msg.Apps)
}

func TestGetProviderOptions_NotConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok(""),
		configNotConnected(),
	)

	req := connect.NewRequest(&observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).GetProviderOptions(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestGetProviderOptions_UnknownProvider(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	req := connect.NewRequest(&observabilityv1.GetProviderOptionsRequest{
		Provider: "not-a-provider",
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).GetProviderOptions(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetProviderOptions_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := connect.NewRequest(&observabilityv1.GetProviderOptionsRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).GetProviderOptions(context.Background(), req)
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
	))

	req := connect.NewRequest(&observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: "o/r"},
			},
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).SetProviderConfig(context.Background(), req)
	require.NoError(t, err)

	_, conn, err := testApp.oauthConnRepo.Get(t.Context(), models.OAuthProviderGithub)
	require.NoError(t, err)
	assert.JSONEq(t, `{"repo":"o/r"}`, string(conn.Config))

	listReq := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(listReq, accessToken)
	listResp, err := observabilityClient(
		t,
	).ListOAuthConnections(context.Background(), listReq)
	require.NoError(t, err)
	for _, c := range listResp.Msg.Connections {
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
	))

	req := connect.NewRequest(&observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderSentry),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Sentry{
				Sentry: &observabilityv1.SentryConfig{
					Org: "org-a", Projects: []string{"p1", "p2"},
				},
			},
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).SetProviderConfig(context.Background(), req)
	require.NoError(t, err)

	_, conn, err := testApp.oauthConnRepo.Get(t.Context(), models.OAuthProviderSentry)
	require.NoError(t, err)
	assert.JSONEq(t, `{"org":"org-a","projects":["p1","p2"]}`, string(conn.Config))
}

func TestSetProviderConfig_DigitalOcean_ParsesFriendlyOption(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderDigitalOcean,
		&oauth2.Token{ //nolint:exhaustruct // other token fields unused in test
			AccessToken: "tok",
		},
		testUserID,
	))

	req := connect.NewRequest(&observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderDigitalOcean),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Digitalocean{
				Digitalocean: &observabilityv1.DigitalOceanConfig{
					AppId: "id-1 — app-one",
				},
			},
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).SetProviderConfig(context.Background(), req)
	require.NoError(t, err)

	_, conn, err := testApp.oauthConnRepo.Get(
		t.Context(),
		models.OAuthProviderDigitalOcean,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"app_id":"id-1"}`, string(conn.Config))
}

func TestSetProviderConfig_MissingFields(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	req := connect.NewRequest(&observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config:   &observabilityv1.ProviderConfig{},
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).SetProviderConfig(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetProviderConfig_NotConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	req := connect.NewRequest(&observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: "o/r"},
			},
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).SetProviderConfig(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSetProviderConfig_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := connect.NewRequest(&observabilityv1.SetProviderConfigRequest{
		Provider: string(models.OAuthProviderGithub),
		Config: &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: "o/r"},
			},
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).SetProviderConfig(context.Background(), req)
	requirePermissionDenied(t, err)
}
