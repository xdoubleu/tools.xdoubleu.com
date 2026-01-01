package goaltracker_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/XDoubleU/essentia/pkg/test"
	"github.com/stretchr/testify/assert"
)

func TestRefreshProgressHandler(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(testApp.GetName(), http.NewServeMux()),
		http.MethodGet,
		fmt.Sprintf("%s/api/progress/0/refresh", testApp.GetName()),
	)

	tReq.AddCookie(&accessToken)
	tReq.AddCookie(&refreshToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
