package learningpaths

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *LearningPaths) Routes(prefix string, mux *http.ServeMux) {
	learningPathsPath, learningPathsHandler := learningpathsv1connect.
		NewLearningPathsServiceHandler(
			&learningPathsConnectHandler{app: a},
			iapp.ScrubInternalErrors(a.Logger),
		)
	mux.Handle(
		fmt.Sprintf("POST %s", learningPathsPath),
		a.services.Auth.AppAccess(prefix, learningPathsHandler.ServeHTTP),
	)
}
