package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
)

func TestVersionEndpointSuccess(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/api/version",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rs.Body).Decode(&response))
	assert.Contains(t, response, "release")
	assert.NotEmpty(t, response["release"])
}

func TestVersionEndpointNoAuth(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/api/version",
	)
	// Intentionally not adding auth cookie

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rs.Body).Decode(&response))
	assert.Contains(t, response, "release")
}

func TestVersionEndpointPostNotAllowed(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/api/version",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusMethodNotAllowed, rs.StatusCode)
}

func TestVersionEndpointWithRelease(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/api/version",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rs.Body).Decode(&response))
	// Verify the release value matches the config
	assert.Equal(t, testApp.config.Release, response["release"])
}
