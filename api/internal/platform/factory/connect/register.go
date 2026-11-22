package connect

import (
	connectv2 "connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
)

// RegisterAdmin registers the services an admin's browser session or agent
// reaches: the board, the repos, runner pairing and chats.
func RegisterAdmin(s *connectv2.Server, d *Deps) {
	factoryv1connect.RegisterTaskServiceHandler(s, &taskConnectHandler{app: d})
	factoryv1connect.RegisterRepoServiceHandler(s, &repoConnectHandler{app: d})
	factoryv1connect.RegisterRunnerAdminServiceHandler(s, &runnerAdminHandler{app: d})
	factoryv1connect.RegisterChatServiceHandler(s, &chatConnectHandler{app: d})
}

// RegisterRunner registers the runner's own service, which authenticates by
// device token.
func RegisterRunner(s *connectv2.Server, d *Deps) {
	factoryv1connect.RegisterRunnerServiceHandler(s, &runnerHandler{app: d})
}
