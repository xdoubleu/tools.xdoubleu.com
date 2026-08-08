package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/testhelper"
)

func TestSecurityHeaders(t *testing.T) {
	tReq := testhelper.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/api/version",
	)

	rs := tReq.Do(t)
	assert.Equal(
		t,
		"max-age=31536000; includeSubDomains",
		rs.Header.Get("Strict-Transport-Security"),
	)
	assert.Equal(t, "DENY", rs.Header.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rs.Header.Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rs.Header.Get("Content-Security-Policy"))
}
